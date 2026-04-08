package proto

import "zone.com/internal/wire"

// Hearts message struct sizes (MSVC x86 default alignment)
const (
	HeartsStartGameSize   = 40 // uint16*4 + uint32*2 + uint32[6]
	HeartsReplacePlayerSz = 8  // uint32 + int16 + int16
	HeartsStartHandSize   = 20 // int16 + char[18]
	HeartsStartPlaySize   = 4  // int16 + int16
	HeartsEndHandSize     = 14 // int16[6] + int16
	HeartsEndGameSize     = 4  // int16 + uint16
	HeartsClientReadySize = 16 // uint32*3 + int16 + int16
	HeartsPassCardsSize   = 8  // int16 + char[5] + 1 pad
	HeartsPlayCardSize    = 4  // int16 + char + uchar
	HeartsNewGameSize     = 2  // int16
	HeartsTalkHdrSize     = 8  // uint32 + int16 + uint16
	HeartsCheckInSize     = 8  // uint32 + int16 + 2 pad
	HeartsOptionsSize     = 8  // int16 + int16 + uint32
)

// HeartsClientReady is sent by each client on launch.
type HeartsClientReady struct {
	ProtocolSignature uint32
	ProtocolVersion   uint32
	Version           uint32
	Seat              int16
}

func (m *HeartsClientReady) Unmarshal(b []byte) {
	m.ProtocolSignature = wire.ReadBE32(b[0:])
	m.ProtocolVersion = wire.ReadBE32(b[4:])
	m.Version = wire.ReadBE32(b[8:])
	m.Seat = int16(wire.ReadBE16(b[12:]))
}

// HeartsStartGame is sent by the server to all clients once all are ready.
type HeartsStartGame struct {
	NumCardsInHand  uint16
	NumCardsInPass  uint16
	NumPointsInGame uint16
	GameOptions     uint32
	Players         [HeartsMaxNumPlayers]uint32
}

func (m *HeartsStartGame) Marshal() []byte {
	b := make([]byte, HeartsStartGameSize)
	wire.WriteBE16(b[0:], m.NumCardsInHand)
	wire.WriteBE16(b[2:], m.NumCardsInPass)
	wire.WriteBE16(b[4:], m.NumPointsInGame)
	// b[6:8] rfu1
	wire.WriteBE32(b[8:], m.GameOptions)
	// b[12:16] rfu2
	for i := 0; i < HeartsMaxNumPlayers; i++ {
		wire.WriteBE32(b[16+i*4:], m.Players[i])
	}
	return b
}

// HeartsStartHand is sent per-player with their dealt cards.
type HeartsStartHand struct {
	PassDir int16
	Cards   [HeartsMaxNumCardsInHand]byte
}

func (m *HeartsStartHand) Marshal() []byte {
	b := make([]byte, HeartsStartHandSize)
	wire.WriteBE16(b[0:], uint16(m.PassDir))
	copy(b[2:], m.Cards[:])
	return b
}

// HeartsStartPlay tells clients who plays first.
type HeartsStartPlay struct {
	Seat int16
}

func (m *HeartsStartPlay) Marshal() []byte {
	b := make([]byte, HeartsStartPlaySize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	// b[2:4] rfu
	return b
}

// HeartsPassCards is sent by each client with their 3 passed cards.
type HeartsPassCards struct {
	Seat int16
	Pass [HeartsMaxNumCardsInPass]byte
}

func (m *HeartsPassCards) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	copy(m.Pass[:], b[2:2+HeartsMaxNumCardsInPass])
}

func (m *HeartsPassCards) Marshal() []byte {
	b := make([]byte, HeartsPassCardsSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	copy(b[2:], m.Pass[:])
	return b
}

// HeartsPlayCard is sent by the active player and broadcast by the server.
type HeartsPlayCard struct {
	Seat int16
	Card byte
}

func (m *HeartsPlayCard) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Card = b[2]
}

func (m *HeartsPlayCard) Marshal() []byte {
	b := make([]byte, HeartsPlayCardSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	b[2] = m.Card
	return b
}

// HeartsEndHand is sent after all 13 tricks with per-player scores.
type HeartsEndHand struct {
	Score     [HeartsMaxNumPlayers]int16
	RunPlayer int16
}

func (m *HeartsEndHand) Marshal() []byte {
	b := make([]byte, HeartsEndHandSize)
	for i := 0; i < HeartsMaxNumPlayers; i++ {
		wire.WriteBE16(b[i*2:], uint16(m.Score[i]))
	}
	wire.WriteBE16(b[12:], uint16(m.RunPlayer))
	return b
}

// HeartsEndGame is sent when a player reaches the point threshold.
type HeartsEndGame struct {
	Forfeiter int16
	Timeout   uint16
}

func (m *HeartsEndGame) Marshal() []byte {
	b := make([]byte, HeartsEndGameSize)
	wire.WriteBE16(b[0:], uint16(m.Forfeiter))
	wire.WriteBE16(b[2:], m.Timeout)
	return b
}

// HeartsNewGame is the rematch vote from a client.
type HeartsNewGame struct {
	Seat int16
}

func (m *HeartsNewGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
}

// HeartsTalk is the chat message header (text follows immediately).
type HeartsTalk struct {
	UserID     uint32
	Seat       int16
	MessageLen uint16
}

func (m *HeartsTalk) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.MessageLen = wire.ReadBE16(b[6:])
}

func (m *HeartsTalk) Marshal() []byte {
	b := make([]byte, HeartsTalkHdrSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteBE16(b[6:], m.MessageLen)
	return b
}

// HeartsCheckIn is the player connection acknowledgement.
type HeartsCheckIn struct {
	UserID uint32
	Seat   int16
}

func (m *HeartsCheckIn) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
}

func (m *HeartsCheckIn) Marshal() []byte {
	b := make([]byte, HeartsCheckInSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	return b
}

// HeartsOptions is the player options message.
type HeartsOptions struct {
	Seat    int16
	Options uint32
}

func (m *HeartsOptions) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	// b[2:4] rfu
	m.Options = wire.ReadBE32(b[4:])
}
