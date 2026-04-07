package conn

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// Conn wraps a TCP connection with the Zone connection layer protocol.
type Conn struct {
	raw    net.Conn
	reader *wire.FrameReader
	writer *wire.FrameWriter
	mu     sync.Mutex // serializes writes
	closed bool
}

// ServerHandshake performs the server-side connection handshake:
// reads Hi, validates, generates key, sends Hello.
func ServerHandshake(raw net.Conn) (*Conn, error) {
	addr := raw.RemoteAddr()
	log.Printf("[conn] %s: starting handshake (30s deadline)", addr)
	raw.SetDeadline(time.Now().Add(30 * time.Second))
	defer raw.SetDeadline(time.Time{})

	// Read Hi message (encrypted with DefaultKey)
	fr := wire.NewFrameReader(raw, 0, 0) // temp reader
	hi, err := fr.ReadHi()
	if err != nil {
		return nil, fmt.Errorf("handshake read hi: %w", err)
	}

	log.Printf("[conn] %s: Hi received: sig=%08x ver=%d type=%d product=%08x(%s)",
		addr, hi.Header.Signature, hi.ProtocolVersion, hi.Header.Type,
		hi.ProductSignature, sigToString(hi.ProductSignature))
	log.Printf("[conn] %s: Hi options: mask=%08x flags=%08x clientKey=%08x",
		addr, hi.OptionFlagsMask, hi.OptionFlags, hi.ClientKey)
	log.Printf("[conn] %s: Hi GUID: %x", addr, hi.MachineGUID)

	// Validate
	if hi.Header.Signature != wire.ConnSig {
		return nil, fmt.Errorf("bad signature: %08x", hi.Header.Signature)
	}
	if hi.ProtocolVersion != wire.ConnVersion {
		return nil, fmt.Errorf("bad version: %d", hi.ProtocolVersion)
	}
	if hi.Header.Type != wire.MsgTypeHi {
		return nil, fmt.Errorf("bad type: %d", hi.Header.Type)
	}
	if hi.ProductSignature != proto.ProductSigFree && hi.ProductSignature != proto.ProductSigZone {
		return nil, fmt.Errorf("bad product sig: %08x", hi.ProductSignature)
	}

	// Negotiate options
	optFlags := hi.OptionFlagsMask & hi.OptionFlags
	log.Printf("[conn] %s: negotiated optFlags=%08x (aggGeneric=%v clientKey=%v)",
		addr, optFlags,
		optFlags&wire.OptionAggGeneric != 0,
		optFlags&wire.OptionClientKey != 0)

	// Generate session key: one random byte replicated 4 times
	// Matches coninfo.cpp:1239-1243
	var sessionKey uint32
	useClientKey := (hi.OptionFlagsMask&wire.OptionClientKey) != 0 &&
		(hi.OptionFlags&wire.OptionClientKey) != 0
	if useClientKey {
		sessionKey = hi.ClientKey
		log.Printf("[conn] %s: using CLIENT key: %08x", addr, sessionKey)
	} else {
		kb := byte(time.Now().UnixNano())
		if kb == 0 {
			kb = 0x42 // avoid zero key
		}
		sessionKey = uint32(kb) | uint32(kb)<<8 | uint32(kb)<<16 | uint32(kb)<<24
		log.Printf("[conn] %s: generated SERVER key: byte=0x%02x key=%08x", addr, kb, sessionKey)
	}

	// Build and send Hello
	hello := &wire.HelloMsg{
		FirstSequenceID: 1,
		Key:             sessionKey,
		OptionFlags:     optFlags,
	}
	copy(hello.MachineGUID[:], hi.MachineGUID[:])

	log.Printf("[conn] %s: sending Hello: firstSeq=%d key=%08x optFlags=%08x",
		addr, hello.FirstSequenceID, hello.Key, hello.OptionFlags)

	fw := wire.NewFrameWriter(raw, 0, 0) // temp writer
	if err := fw.WriteHello(hello); err != nil {
		return nil, fmt.Errorf("handshake write hello: %w", err)
	}

	log.Printf("[conn] %s: handshake complete, session key=%08x opts=%08x", addr, sessionKey, optFlags)

	c := &Conn{
		raw:    raw,
		reader: wire.NewFrameReader(raw, sessionKey, 1),
		writer: wire.NewFrameWriter(raw, sessionKey, 1),
	}
	return c, nil
}

func sigToString(sig uint32) string {
	b := [4]byte{byte(sig >> 24), byte(sig >> 16), byte(sig >> 8), byte(sig)}
	for i := range b {
		if b[i] < 0x20 || b[i] > 0x7e {
			b[i] = '?'
		}
	}
	return string(b[:])
}

// ReadAppMessages reads the next frame and returns app messages.
func (c *Conn) ReadAppMessages() ([]wire.AppMessage, error) {
	for {
		log.Printf("[conn] %s: reading next frame...", c.raw.RemoteAddr())
		msgs, ping, err := c.reader.ReadNextFrame()
		if err != nil {
			log.Printf("[conn] %s: read error: %v", c.raw.RemoteAddr(), err)
			return nil, err
		}
		if ping != nil {
			log.Printf("[conn] %s: control ping received yourClk=%08x myClk=%08x", c.raw.RemoteAddr(), ping.YourClk, ping.MyClk)
			if err := c.writePingResponse(ping.YourClk); err != nil {
				return nil, err
			}
			continue
		}
		if msgs == nil {
			log.Printf("[conn] %s: keepalive received", c.raw.RemoteAddr())
			continue
		}
		msgs, handled, err := c.handleInternalAppMessages(msgs)
		if err != nil {
			return nil, err
		}
		if handled && len(msgs) == 0 {
			continue
		}
		log.Printf("[conn] %s: received %d app message(s)", c.raw.RemoteAddr(), len(msgs))
		return msgs, nil
	}
}

// WriteAppMessages sends app messages in a single Generic frame.
func (c *Conn) WriteAppMessages(msgs []wire.AppMessage) error {
	log.Printf("[conn] %s: writing %d app message(s)", c.raw.RemoteAddr(), len(msgs))
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.WriteGeneric(msgs)
}

// SendAppMessage sends a single app message.
func (c *Conn) SendAppMessage(sig, channel, msgType uint32, data []byte) error {
	log.Printf("[conn] %s: SendAppMessage sig=%08x ch=%d type=%d datalen=%d",
		c.raw.RemoteAddr(), sig, channel, msgType, len(data))
	return c.WriteAppMessages([]wire.AppMessage{{
		Signature: sig,
		Channel:   channel,
		Type:      msgType,
		Data:      data,
	}})
}

// RemoteAddr returns the remote address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.raw.Close()
}

func (c *Conn) handleInternalAppMessages(msgs []wire.AppMessage) ([]wire.AppMessage, bool, error) {
	out := make([]wire.AppMessage, 0, len(msgs))
	handled := false
	for _, msg := range msgs {
		if msg.Signature != proto.InternalAppSig {
			out = append(out, msg)
			continue
		}
		handled = true
		log.Printf("[conn] %s: internal app msg type=%08x ch=%d datalen=%d",
			c.raw.RemoteAddr(), msg.Type, msg.Channel, len(msg.Data))
		switch msg.Type {
		case proto.ConnectionKeepAlive:
			log.Printf("[conn] %s: internal keepalive", c.raw.RemoteAddr())
		case proto.ConnectionPing:
			log.Printf("[conn] %s: internal ping", c.raw.RemoteAddr())
			if err := c.SendAppMessage(proto.InternalAppSig, 0, proto.ConnectionPingReply, []byte{0, 0, 0, 0}); err != nil {
				return nil, handled, err
			}
		case proto.ConnectionPingReply:
			log.Printf("[conn] %s: internal ping reply", c.raw.RemoteAddr())
		default:
			log.Printf("[conn] %s: unknown internal app msg type=%08x", c.raw.RemoteAddr(), msg.Type)
		}
	}
	return out, handled, nil
}

func (c *Conn) writePingResponse(yourClk uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.WritePingResponse(yourClk)
}

// Raw returns the underlying net.Conn for deadline setting etc.
func (c *Conn) Raw() io.ReadWriteCloser {
	return c.raw
}
