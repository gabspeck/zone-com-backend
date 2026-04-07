package wire

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sync"
)

// FrameReader reads connection-layer frames from a TCP stream.
type FrameReader struct {
	r   io.Reader
	key uint32
	seq uint32
}

func NewFrameReader(r io.Reader, key uint32, initialSeq uint32) *FrameReader {
	return &FrameReader{r: r, key: key, seq: initialSeq}
}

// ReadHi reads the initial Hi message (encrypted with DefaultKey).
func (fr *FrameReader) ReadHi() (*HiMsg, error) {
	log.Printf("[wire] ReadHi: reading %d bytes...", HiMsgSize)
	buf := make([]byte, HiMsgSize)
	if _, err := io.ReadFull(fr.r, buf); err != nil {
		return nil, fmt.Errorf("read hi: %w", err)
	}
	log.Printf("[wire] ReadHi: encrypted:\n%s", hex.Dump(buf))
	Decrypt(buf, DefaultKey)
	log.Printf("[wire] ReadHi: decrypted:\n%s", hex.Dump(buf))

	msg := &HiMsg{}
	if err := msg.Unmarshal(buf); err != nil {
		return nil, err
	}
	log.Printf("[wire] ReadHi: sig=%08x totalLen=%d type=%d intLen=%d ver=%d product=%08x optMask=%08x optFlags=%08x clientKey=%08x guid=%x",
		msg.Header.Signature, msg.Header.TotalLength, msg.Header.Type, msg.Header.IntLength,
		msg.ProtocolVersion, msg.ProductSignature, msg.OptionFlagsMask, msg.OptionFlags,
		msg.ClientKey, msg.MachineGUID)
	return msg, nil
}

// ReadNextFrame reads the next post-handshake frame.
// For Generic frames it returns app messages.
// For KeepAlive or Ping control frames it returns ctrl and nil app messages.
func (fr *FrameReader) ReadNextFrame() ([]AppMessage, *PingMsg, error) {
	hdrBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(fr.r, hdrBuf); err != nil {
		return nil, nil, fmt.Errorf("read frame header: %w", err)
	}

	log.Printf("[wire] ReadNextFrame: encrypted header:\n%s", hex.Dump(hdrBuf))

	if fr.key != 0 {
		Decrypt(hdrBuf, fr.key)
	}

	var hdr Header
	hdr.Unmarshal(hdrBuf[0:])

	if hdr.Signature != ConnSig {
		return nil, nil, fmt.Errorf("bad frame signature: %08x", hdr.Signature)
	}
	if hdr.TotalLength < HeaderSize || hdr.IntLength < HeaderSize || uint32(hdr.IntLength) > hdr.TotalLength {
		return nil, nil, fmt.Errorf("bad frame lengths: total=%d int=%d", hdr.TotalLength, hdr.IntLength)
	}

	remaining := int(hdr.TotalLength) - HeaderSize
	rest := make([]byte, remaining)
	if _, err := io.ReadFull(fr.r, rest); err != nil {
		return nil, nil, fmt.Errorf("read frame body: %w", err)
	}

	switch hdr.Type {
	case MsgTypeGeneric:
		intHdrRest := append([]byte(nil), rest[:GenericHdrSize-HeaderSize]...)
		if fr.key != 0 {
			Decrypt(intHdrRest, fr.key)
		}
		fullHdr := append(hdrBuf, intHdrRest...)
		return fr.parseGenericFrame(fullHdr, rest[GenericHdrSize-HeaderSize:])
	case MsgTypeKeepAliv:
		log.Printf("[wire] ReadNextFrame: received keepalive")
		return nil, nil, nil
	case MsgTypePing:
		body := rest
		if fr.key != 0 {
			Decrypt(body, fr.key)
		}
		if len(body) < 8 {
			return nil, nil, fmt.Errorf("ping frame too short: %d", len(body))
		}
		msg := &PingMsg{
			Header:  hdr,
			YourClk: ReadLE32(body[0:]),
			MyClk:   ReadLE32(body[4:]),
		}
		log.Printf("[wire] ReadNextFrame: received ping yourClk=%08x myClk=%08x", msg.YourClk, msg.MyClk)
		return nil, msg, nil
	default:
		return nil, nil, fmt.Errorf("unsupported frame type %d", hdr.Type)
	}
}

// ReadGeneric reads the next Generic frame and returns decrypted app messages.
func (fr *FrameReader) ReadGeneric() ([]AppMessage, error) {
	msgs, ctrl, err := fr.ReadNextFrame()
	if err != nil {
		return nil, err
	}
	if ctrl != nil {
		return nil, fmt.Errorf("expected generic msg type 0, got %d", ctrl.Header.Type)
	}
	if msgs == nil {
		return nil, fmt.Errorf("expected generic msg type 0, got %d", MsgTypeKeepAliv)
	}
	return msgs, nil
}

func (fr *FrameReader) parseGenericFrame(hdrBuf []byte, rest []byte) ([]AppMessage, *PingMsg, error) {
	seqID := ReadLE32(hdrBuf[12:])
	checksum := ReadLE32(hdrBuf[16:])

	var hdr Header
	hdr.Unmarshal(hdrBuf[0:])

	log.Printf("[wire] ReadGeneric: sig=%08x totalLen=%d type=%d intLen=%d seq=%d checksum=%08x",
		hdr.Signature, hdr.TotalLength, hdr.Type, hdr.IntLength, seqID, checksum)

	if fr.seq != 0 && seqID != fr.seq {
		return nil, nil, fmt.Errorf("sequence mismatch: got %d, want %d", seqID, fr.seq)
	}
	fr.seq = seqID + 1

	if len(rest) < GenericFooterSize {
		return nil, nil, fmt.Errorf("generic frame too small: remaining=%d", len(rest))
	}

	// Footer is the last 4 bytes, NOT encrypted
	footerOff := len(rest) - GenericFooterSize
	footerStatus := ReadLE32(rest[footerOff:])
	log.Printf("[wire] ReadGeneric: footer status=%d (at offset %d of %d remaining bytes)", footerStatus, footerOff, len(rest))
	if footerStatus != GenericStatusOk {
		return nil, nil, fmt.Errorf("generic footer status: %d", footerStatus)
	}

	// Decrypt app data (everything before footer)
	appData := rest[:footerOff]
	if fr.key != 0 {
		Decrypt(appData, fr.key)
	}

	log.Printf("[wire] ReadGeneric: decrypted app data (%d bytes):\n%s", len(appData), hex.Dump(appData))

	// Verify checksum
	computed := Checksum(appData)
	if computed != checksum {
		return nil, nil, fmt.Errorf("checksum mismatch: got %08x, want %08x", computed, checksum)
	}
	log.Printf("[wire] ReadGeneric: checksum OK (%08x)", checksum)

	msgs, err := ParseAppMessages(appData)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("[wire] ReadGeneric: parsed %d app message(s)", len(msgs))
	for i, m := range msgs {
		log.Printf("[wire]   msg[%d]: sig=%08x ch=%d type=%d datalen=%d data=%s",
			i, m.Signature, m.Channel, m.Type, len(m.Data), hex.EncodeToString(m.Data))
	}
	return msgs, nil, nil
}

// FrameWriter writes connection-layer frames to a TCP stream.
type FrameWriter struct {
	w   io.Writer
	key uint32
	seq uint32
	mu  sync.Mutex
}

func NewFrameWriter(w io.Writer, key uint32, initialSeq uint32) *FrameWriter {
	return &FrameWriter{w: w, key: key, seq: initialSeq}
}

// WriteHello writes the Hello response (encrypted with DefaultKey).
func (fw *FrameWriter) WriteHello(msg *HelloMsg) error {
	log.Printf("[wire] WriteHello: firstSeqID=%d key=%08x optFlags=%08x guid=%x",
		msg.FirstSequenceID, msg.Key, msg.OptionFlags, msg.MachineGUID)
	buf := msg.Marshal()
	log.Printf("[wire] WriteHello: plaintext:\n%s", hex.Dump(buf))
	Encrypt(buf, DefaultKey)
	log.Printf("[wire] WriteHello: encrypted (%d bytes):\n%s", len(buf), hex.Dump(buf))
	_, err := fw.w.Write(buf)
	return err
}

func (fw *FrameWriter) WriteKeepAlive() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	hdrBuf := make([]byte, HeaderSize)
	hdr := Header{
		Signature:   ConnSig,
		TotalLength: HeaderSize,
		Type:        MsgTypeKeepAliv,
		IntLength:   HeaderSize,
	}
	hdr.Marshal(hdrBuf)
	if fw.key != 0 {
		Encrypt(hdrBuf, fw.key)
	}
	log.Printf("[wire] WriteKeepAlive: writing %d bytes", len(hdrBuf))
	_, err := fw.w.Write(hdrBuf)
	return err
}

func (fw *FrameWriter) WritePingResponse(yourClk uint32) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	hdrBuf := make([]byte, HeaderSize)
	hdr := Header{
		Signature:   ConnSig,
		TotalLength: HeaderSize + 8,
		Type:        MsgTypePing,
		IntLength:   HeaderSize + 8,
	}
	hdr.Marshal(hdrBuf)
	body := make([]byte, 8)
	WriteLE32(body[0:], yourClk)
	WriteLE32(body[4:], 0)
	if fw.key != 0 {
		Encrypt(hdrBuf, fw.key)
		Encrypt(body, fw.key)
	}
	buf := append(hdrBuf, body...)
	log.Printf("[wire] WritePingResponse: yourClk=%08x writing %d bytes", yourClk, len(buf))
	_, err := fw.w.Write(buf)
	return err
}

// WriteGeneric wraps app messages in a Generic frame, encrypts, and writes.
func (fw *FrameWriter) WriteGeneric(msgs []AppMessage) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	log.Printf("[wire] WriteGeneric: %d message(s), seq=%d", len(msgs), fw.seq)
	for i, m := range msgs {
		log.Printf("[wire]   msg[%d]: sig=%08x ch=%d type=%d datalen=%d data=%s",
			i, m.Signature, m.Channel, m.Type, len(m.Data), hex.EncodeToString(m.Data))
	}

	// Build app payload: AppHeader + data for each message (no per-message padding)
	// C source coninfo.cpp:1853 aggregate mode: buflen = len + sizeof(AppHeader)
	var appData []byte
	for _, m := range msgs {
		hdr := make([]byte, AppHeaderSize)
		m.MarshalAppHeader(hdr)
		appData = append(appData, hdr...)
		appData = append(appData, m.Data...)
	}

	rawLen := len(appData)

	// Pad appData so (appData + footer) is 4-byte aligned
	// Matching C: ZRoundUpLenWFooter(n) = (n + 3 + sizeof(Footer)) & ~0x3
	bodyLen := (len(appData) + 3 + GenericFooterSize) & ^3
	if pad := bodyLen - GenericFooterSize - len(appData); pad > 0 {
		log.Printf("[wire] WriteGeneric: padding app data %d -> %d bytes (+%d pad)", rawLen, rawLen+pad, pad)
		appData = append(appData, make([]byte, pad)...)
	}

	// Checksum covers padded app data (C: ZSecurityChecksum(pBuf+sizeof(GenericMsg), len-sizeof(Footer)))
	checksum := Checksum(appData)

	// Build generic header
	totalLen := GenericHdrSize + len(appData) + GenericFooterSize
	hdrBuf := make([]byte, GenericHdrSize)
	hdr := Header{
		Signature:   ConnSig,
		TotalLength: uint32(totalLen),
		Type:        MsgTypeGeneric,
		IntLength:   GenericHdrSize,
	}
	hdr.Marshal(hdrBuf)
	WriteLE32(hdrBuf[12:], fw.seq)
	WriteLE32(hdrBuf[16:], checksum)

	log.Printf("[wire] WriteGeneric: totalLen=%d appData=%d seq=%d checksum=%08x",
		totalLen, len(appData), fw.seq, checksum)
	log.Printf("[wire] WriteGeneric: plaintext app data:\n%s", hex.Dump(appData))

	fw.seq++

	// Build footer (NOT encrypted)
	footer := make([]byte, GenericFooterSize)
	WriteLE32(footer, GenericStatusOk)

	// Encrypt header and app data separately
	if fw.key != 0 {
		Encrypt(hdrBuf, fw.key)
		Encrypt(appData, fw.key)
	}

	// Write all parts
	buf := make([]byte, 0, totalLen)
	buf = append(buf, hdrBuf...)
	buf = append(buf, appData...)
	buf = append(buf, footer...)
	log.Printf("[wire] WriteGeneric: writing %d bytes total", len(buf))
	_, err := fw.w.Write(buf)
	return err
}
