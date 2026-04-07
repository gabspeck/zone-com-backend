package wire

import "fmt"

// Sizes matching C sizeof with #pragma pack(push, 4)
const (
	HeaderSize        = 12 // ZConnInternalHeader
	HiMsgSize         = 48 // ZConnInternalHiMsg
	HelloMsgSize      = 40 // ZConnInternalHelloMsg
	GenericHdrSize    = 20 // ZConnInternalGenericMsg (header only, no app data)
	GenericFooterSize = 4  // ZConnInternalGenericFooter
	AppHeaderSize     = 16 // ZConnInternalAppHeader
)

// Connection layer message types
const (
	MsgTypeGeneric  = 0
	MsgTypeHi       = 1
	MsgTypeHello    = 2
	MsgTypeGoodbye  = 3
	MsgTypeKeepAliv = 4
	MsgTypePing     = 5
)

// Connection layer constants
const (
	ConnSig     uint32 = 0x4C694E6B // 'LiNk'
	ConnVersion uint32 = 3

	OptionAggGeneric uint32 = 0x01
	OptionClientKey  uint32 = 0x02

	GenericStatusOk uint32 = 1
)

// Header is ZConnInternalHeader (12 bytes, all LE).
type Header struct {
	Signature   uint32
	TotalLength uint32
	Type        uint16
	IntLength   uint16
}

func (h *Header) Marshal(b []byte) {
	WriteLE32(b[0:], h.Signature)
	WriteLE32(b[4:], h.TotalLength)
	WriteLE16(b[8:], h.Type)
	WriteLE16(b[10:], h.IntLength)
}

func (h *Header) Unmarshal(b []byte) {
	h.Signature = ReadLE32(b[0:])
	h.TotalLength = ReadLE32(b[4:])
	h.Type = ReadLE16(b[8:])
	h.IntLength = ReadLE16(b[10:])
}

// HiMsg is ZConnInternalHiMsg (48 bytes, all LE).
type HiMsg struct {
	Header           Header
	ProtocolVersion  uint32
	ProductSignature uint32
	OptionFlagsMask  uint32
	OptionFlags      uint32
	ClientKey        uint32
	MachineGUID      [16]byte
}

func (m *HiMsg) Unmarshal(b []byte) error {
	if len(b) < HiMsgSize {
		return fmt.Errorf("HiMsg: need %d bytes, got %d", HiMsgSize, len(b))
	}
	m.Header.Unmarshal(b[0:])
	m.ProtocolVersion = ReadLE32(b[12:])
	m.ProductSignature = ReadLE32(b[16:])
	m.OptionFlagsMask = ReadLE32(b[20:])
	m.OptionFlags = ReadLE32(b[24:])
	m.ClientKey = ReadLE32(b[28:])
	copy(m.MachineGUID[:], b[32:48])
	return nil
}

// HelloMsg is ZConnInternalHelloMsg (40 bytes, all LE).
type HelloMsg struct {
	Header          Header
	FirstSequenceID uint32
	Key             uint32
	OptionFlags     uint32
	MachineGUID     [16]byte
}

func (m *HelloMsg) Marshal() []byte {
	b := make([]byte, HelloMsgSize)
	m.Header.Signature = ConnSig
	m.Header.Type = MsgTypeHello
	m.Header.IntLength = HelloMsgSize
	m.Header.TotalLength = HelloMsgSize
	m.Header.Marshal(b[0:])
	WriteLE32(b[12:], m.FirstSequenceID)
	WriteLE32(b[16:], m.Key)
	WriteLE32(b[20:], m.OptionFlags)
	copy(b[24:], m.MachineGUID[:])
	return b
}

// PingMsg is the optional connection-layer ping control frame.
type PingMsg struct {
	Header  Header
	YourClk uint32
	MyClk   uint32
}

// AppMessage is an application-level message extracted from a Generic frame.
type AppMessage struct {
	Signature uint32
	Channel   uint32
	Type      uint32
	Data      []byte
}

// MarshalAppHeader writes the 16-byte app header for this message.
func (m *AppMessage) MarshalAppHeader(b []byte) {
	WriteLE32(b[0:], m.Signature)
	WriteLE32(b[4:], m.Channel)
	WriteLE32(b[8:], m.Type)
	WriteLE32(b[12:], uint32(len(m.Data)))
}

// ParseAppMessages extracts app messages from decrypted generic frame payload.
func ParseAppMessages(data []byte) ([]AppMessage, error) {
	var msgs []AppMessage
	for len(data) >= AppHeaderSize {
		sig := ReadLE32(data[0:])
		ch := ReadLE32(data[4:])
		typ := ReadLE32(data[8:])
		dlen := ReadLE32(data[12:])
		data = data[AppHeaderSize:]

		if uint32(len(data)) < dlen {
			return nil, fmt.Errorf("app message truncated: need %d, have %d", dlen, len(data))
		}

		payload := make([]byte, dlen)
		copy(payload, data[:dlen])
		msgs = append(msgs, AppMessage{
			Signature: sig,
			Channel:   ch,
			Type:      typ,
			Data:      payload,
		})

		// Advance past data — no per-message padding
		// (C source coninfo.cpp:1533: dwApplicationLen -= sizeof(*pHeader) + pHeader->dwDataLength)
		data = data[dlen:]
	}
	return msgs, nil
}

// RoundUp4 rounds n up to the next multiple of 4.
func RoundUp4(n int) int {
	return (n + 3) & ^3
}
