package proto

import "zone.com/internal/wire"

// Spades message struct sizes (MSVC x86 default alignment)
const (
	SpadesClientReadySize = 20 // uint32*4 + int16 + int16(pad)
	SpadesStartGameSize   = 24 // uint32[4] + uint32 + int16 + int16
	SpadesReplacePlayerSize = 8  // uint32 + int16 + int16
	SpadesStartBidSize    = 18 // int16 + int16 + char[13] + 1(pad)
	SpadesStartPlaySize   = 2  // int16
	SpadesEndHandSize     = 36 // int16[2](BE) + ZHandScore(32 bytes, LE)
	SpadesEndGameSize     = 4  // char[4]
	SpadesBidSize         = 6  // int16 + int16 + char + 1(pad)
	SpadesPlaySize        = 6  // int16 + int16 + char + 1(pad)
	SpadesNewGameSize     = 2  // int16
	SpadesTalkHdrSize     = 8  // uint32 + uint16 + int16
	SpadesShownCardsSize  = 2  // int16
	SpadesOptionsSize     = 8  // int16 + int16 + uint32
)

// SpadesClientReady is sent by each client on launch.
type SpadesClientReady struct {
	ProtocolSignature uint32
	ProtocolVersion   uint32
	Version           uint32
	PlayerID          uint32
	Seat              int16
}

func (m *SpadesClientReady) Unmarshal(b []byte) {
	m.ProtocolSignature = wire.ReadBE32(b[0:])
	m.ProtocolVersion = wire.ReadBE32(b[4:])
	m.Version = wire.ReadBE32(b[8:])
	m.PlayerID = wire.ReadBE32(b[12:])
	m.Seat = int16(wire.ReadBE16(b[16:]))
}

// SpadesStartGame is sent by the server to all clients once all are ready.
type SpadesStartGame struct {
	Players         [SpadesNumPlayers]uint32
	GameOptions     uint32
	NumPointsInGame int16
	MinPointsInGame int16 // NOT endian-converted; write as LE
}

func (m *SpadesStartGame) Marshal() []byte {
	b := make([]byte, SpadesStartGameSize)
	for i := 0; i < SpadesNumPlayers; i++ {
		wire.WriteBE32(b[i*4:], m.Players[i])
	}
	wire.WriteBE32(b[16:], m.GameOptions)
	wire.WriteBE16(b[20:], uint16(m.NumPointsInGame))
	wire.WriteLE16(b[22:], uint16(m.MinPointsInGame)) // NOT endian-converted
	return b
}

// SpadesStartBid is sent per-player with their dealt cards and bidding info.
type SpadesStartBid struct {
	BoardNumber int16
	Dealer      int16
	Hand        [SpadesNumCardsInHand]byte
}

func (m *SpadesStartBid) Marshal() []byte {
	b := make([]byte, SpadesStartBidSize)
	wire.WriteBE16(b[0:], uint16(m.BoardNumber))
	wire.WriteBE16(b[2:], uint16(m.Dealer))
	copy(b[4:], m.Hand[:])
	return b
}

// SpadesStartPlay tells clients who leads the first trick.
type SpadesStartPlay struct {
	Leader int16
}

func (m *SpadesStartPlay) Marshal() []byte {
	b := make([]byte, SpadesStartPlaySize)
	wire.WriteBE16(b[0:], uint16(m.Leader))
	return b
}

// SpadesHandScore is the scoring breakdown per team (LE fields — not endian-converted).
type SpadesHandScore struct {
	BoardNumber int16
	Scores      [SpadesNumTeams]int16
	Base        [SpadesNumTeams]int16
	BagBonus    [SpadesNumTeams]int16
	Nil         [SpadesNumTeams]int16
	BagPenalty  [SpadesNumTeams]int16
}

// SpadesEndHand is sent after all 13 tricks. Mixed endianness.
type SpadesEndHand struct {
	Bags  [SpadesNumTeams]int16 // BE
	Score SpadesHandScore       // LE (not endian-converted)
}

func (m *SpadesEndHand) Marshal() []byte {
	b := make([]byte, SpadesEndHandSize)
	// bags: BE
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteBE16(b[i*2:], uint16(m.Bags[i]))
	}
	// ZHandScore: all LE
	wire.WriteLE16(b[4:], uint16(m.Score.BoardNumber))
	// b[6:8] rfu
	// b[8:12] bids (char[4], unused — leave zero)
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteLE16(b[12+i*2:], uint16(m.Score.Scores[i]))
	}
	// b[16:20] bonus (unused, leave zero)
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteLE16(b[20+i*2:], uint16(m.Score.Base[i]))
	}
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteLE16(b[24+i*2:], uint16(m.Score.BagBonus[i]))
	}
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteLE16(b[28+i*2:], uint16(m.Score.Nil[i]))
	}
	for i := 0; i < SpadesNumTeams; i++ {
		wire.WriteLE16(b[32+i*2:], uint16(m.Score.BagPenalty[i]))
	}
	return b
}

// SpadesEndGame is sent when a team reaches the point threshold.
type SpadesEndGame struct {
	Winners [SpadesNumPlayers]byte
}

func (m *SpadesEndGame) Marshal() []byte {
	b := make([]byte, SpadesEndGameSize)
	copy(b, m.Winners[:])
	return b
}

// SpadesBid is bidirectional: client sends seat+bid, server broadcasts with nextBidder.
type SpadesBid struct {
	Seat       int16
	NextBidder int16
	Bid        byte
}

func (m *SpadesBid) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.NextBidder = int16(wire.ReadBE16(b[2:]))
	m.Bid = b[4]
}

func (m *SpadesBid) Marshal() []byte {
	b := make([]byte, SpadesBidSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.NextBidder))
	b[4] = m.Bid
	return b
}

// SpadesPlay is bidirectional: client sends seat+card, server broadcasts with nextPlayer.
type SpadesPlay struct {
	Seat       int16
	NextPlayer int16
	Card       byte
}

func (m *SpadesPlay) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.NextPlayer = int16(wire.ReadBE16(b[2:]))
	m.Card = b[4]
}

func (m *SpadesPlay) Marshal() []byte {
	b := make([]byte, SpadesPlaySize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.NextPlayer))
	b[4] = m.Card
	return b
}

// SpadesNewGame is the rematch vote from a client.
type SpadesNewGame struct {
	Seat int16
}

func (m *SpadesNewGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
}

// SpadesTalk is the chat message header (text follows immediately).
type SpadesTalk struct {
	PlayerID   uint32
	MessageLen uint16
}

func (m *SpadesTalk) Unmarshal(b []byte) {
	m.PlayerID = wire.ReadBE32(b[0:])
	m.MessageLen = wire.ReadBE16(b[4:])
}

func (m *SpadesTalk) Marshal() []byte {
	b := make([]byte, SpadesTalkHdrSize)
	wire.WriteBE32(b[0:], m.PlayerID)
	wire.WriteBE16(b[4:], m.MessageLen)
	return b
}

// SpadesReplacePlayer is sent when a player disconnects/reconnects.
type SpadesReplacePlayer struct {
	PlayerID uint32
	Seat     int16
	FPrompt  int16
}

func (m *SpadesReplacePlayer) Marshal() []byte {
	b := make([]byte, SpadesReplacePlayerSize)
	wire.WriteBE32(b[0:], m.PlayerID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteBE16(b[6:], uint16(m.FPrompt))
	return b
}
