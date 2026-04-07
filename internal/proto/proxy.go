package proto

import "zone.com/internal/wire"

// Proxy message sizes (with #pragma pack(push, 4))
const (
	ProxyHeaderSize       = 4  // weType(2) + wLength(2)
	ProxyHiMsgSize        = 76 // Header(4) + Version(4) + Token(64) + ClientVersion(4)
	ProxyHelloMsgSize     = 4  // Header only
	ProxyMillIDMsgSize    = 12 // Header(4) + 3*LANGID(6) + TimeZone(2)
	ProxyMillSettingsSize = 8  // Header(4) + Chat(2) + Stats(2)
	ProxyServiceReqSize   = 28 // Header(4) + Reason(4) + Service(16) + Channel(4)
	ProxyServiceInfoSize  = 40 // Header(4) + Reason(4) + Service(16) + Channel(4) + Flags(4) + Union(8)
)

// ProxyHiMsg is client->server first proxy message.
type ProxyHiMsg struct {
	Type            uint16
	Length          uint16
	ProtocolVersion uint32
	SetupToken      [SetupTokenLen + 1]byte
	ClientVersion   uint32
}

func (m *ProxyHiMsg) Unmarshal(b []byte) {
	m.Type = wire.ReadLE16(b[0:])
	m.Length = wire.ReadLE16(b[2:])
	m.ProtocolVersion = wire.ReadLE32(b[4:])
	copy(m.SetupToken[:], b[8:8+SetupTokenLen+1])
	m.ClientVersion = wire.ReadLE32(b[72:])
}

// ProxyMillIDMsg is client->server language/timezone info.
type ProxyMillIDMsg struct {
	Type            uint16
	Length          uint16
	SysLang         uint16
	UserLang        uint16
	AppLang         uint16
	TimeZoneMinutes int16
}

func (m *ProxyMillIDMsg) Unmarshal(b []byte) {
	m.Type = wire.ReadLE16(b[0:])
	m.Length = wire.ReadLE16(b[2:])
	m.SysLang = wire.ReadLE16(b[4:])
	m.UserLang = wire.ReadLE16(b[6:])
	m.AppLang = wire.ReadLE16(b[8:])
	m.TimeZoneMinutes = int16(wire.ReadLE16(b[10:]))
}

// ProxyServiceRequestMsg is client->server service connect request.
type ProxyServiceRequestMsg struct {
	Type    uint16
	Length  uint16
	Reason  uint32
	Service [InternalNameLen + 1]byte
	Channel uint32
}

func (m *ProxyServiceRequestMsg) Unmarshal(b []byte) {
	m.Type = wire.ReadLE16(b[0:])
	m.Length = wire.ReadLE16(b[2:])
	m.Reason = wire.ReadLE32(b[4:])
	copy(m.Service[:], b[8:8+InternalNameLen+1])
	m.Channel = wire.ReadLE32(b[24:])
}

// MarshalProxyHello builds the ProxyHello response (just a header).
func MarshalProxyHello() []byte {
	b := make([]byte, ProxyHelloMsgSize)
	wire.WriteLE16(b[0:], uint16(ProxyMsgHello))
	wire.WriteLE16(b[2:], ProxyHelloMsgSize)
	return b
}

// MarshalProxyMillSettings builds the MillSettings response.
func MarshalProxyMillSettings(chat, stats uint16) []byte {
	b := make([]byte, ProxyMillSettingsSize)
	wire.WriteLE16(b[0:], uint16(ProxyMsgMillSettings))
	wire.WriteLE16(b[2:], ProxyMillSettingsSize)
	wire.WriteLE16(b[4:], chat)
	wire.WriteLE16(b[6:], stats)
	return b
}

// MarshalProxyServiceInfo builds a ServiceInfo response.
func MarshalProxyServiceInfo(reason uint32, service [InternalNameLen + 1]byte, channel, flags uint32, ip [4]byte, port uint16) []byte {
	b := make([]byte, ProxyServiceInfoSize)
	wire.WriteLE16(b[0:], uint16(ProxyMsgServiceInfo))
	wire.WriteLE16(b[2:], ProxyServiceInfoSize)
	wire.WriteLE32(b[4:], reason)
	copy(b[8:], service[:])
	wire.WriteLE32(b[24:], channel)
	wire.WriteLE32(b[28:], flags)
	copy(b[32:36], ip[:])
	wire.WriteLE16(b[36:], port)
	// Bytes 38..39 are padding in the packed C struct's union area.
	return b
}
