package proto

import "zone.com/internal/wire"

const (
	BackgammonCheckInSize         = 20
	BackgammonGameStateReqSize    = 8
	BackgammonGameStateRespHdr    = 8
	BackgammonTalkHdrSize         = 8
	BackgammonRollRequestSize     = 2
	BackgammonDiceInfoSize        = 16 // MSVC default alignment: 2-byte padding after int16 Value
	BackgammonDiceRollSize        = 36 // seat(2) + pad(2) + DICEINFO(16) + DICEINFO(16)
	BackgammonEndLogSize          = 12
	BackgammonFirstMoveSize       = 8
	BackgammonMoveTimeoutSize     = 72
	BackgammonEndTurnSize         = 2
	BackgammonCheaterSize         = 40 // seat(2) + pad(2) + DICEINFO(16) + DICEINFO(16) + move(2) + pad(2)
	BackgammonTransactionHdrSize  = 16
	BackgammonTransactionItemSize = 12
)

type GameCheckIn struct {
	ProtocolSignature uint32
	ProtocolVersion   uint32
	ClientVersion     uint32
	PlayerID          uint32
	Seat              int16
	PlayerType        int16
}

func (m *GameCheckIn) Unmarshal(b []byte) {
	m.ProtocolSignature = wire.ReadBE32(b[0:])
	m.ProtocolVersion = wire.ReadBE32(b[4:])
	m.ClientVersion = wire.ReadBE32(b[8:])
	m.PlayerID = wire.ReadBE32(b[12:])
	m.Seat = int16(wire.ReadBE16(b[16:]))
	m.PlayerType = int16(wire.ReadBE16(b[18:]))
}

func (m *GameCheckIn) Marshal() []byte {
	b := make([]byte, BackgammonCheckInSize)
	wire.WriteBE32(b[0:], m.ProtocolSignature)
	wire.WriteBE32(b[4:], m.ProtocolVersion)
	wire.WriteBE32(b[8:], m.ClientVersion)
	wire.WriteBE32(b[12:], m.PlayerID)
	wire.WriteBE16(b[16:], uint16(m.Seat))
	wire.WriteBE16(b[18:], uint16(m.PlayerType))
	return b
}

type GameStateRequest struct {
	PlayerID uint32
	Seat     int16
	Rfu      int16
}

func (m *GameStateRequest) Unmarshal(b []byte) {
	m.PlayerID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.Rfu = int16(wire.ReadLE16(b[6:]))
}

func (m *GameStateRequest) Marshal() []byte {
	b := make([]byte, BackgammonGameStateReqSize)
	wire.WriteBE32(b[0:], m.PlayerID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	return b
}

type GameStateResponse struct {
	PlayerID uint32
	Seat     int16
	Rfu      int16
}

func (m *GameStateResponse) Marshal() []byte {
	b := make([]byte, BackgammonGameStateRespHdr)
	wire.WriteBE32(b[0:], m.PlayerID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	return b
}

type BackgammonTalk struct {
	UserID     uint32
	Seat       int16
	MessageLen uint16
}

func (m *BackgammonTalk) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.MessageLen = wire.ReadBE16(b[6:])
}

func (m *BackgammonTalk) Marshal() []byte {
	b := make([]byte, BackgammonTalkHdrSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteBE16(b[6:], m.MessageLen)
	return b
}

type BackgammonRollRequest struct {
	Seat int16
}

func (m *BackgammonRollRequest) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
}

func (m *BackgammonRollRequest) Marshal() []byte {
	b := make([]byte, BackgammonRollRequestSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	return b
}

type BackgammonDiceInfo struct {
	Value        int16
	EncodedValue int32
	EncoderMul   int16
	EncoderAdd   int16
	NumUses      int32
}

func (m *BackgammonDiceInfo) Unmarshal(b []byte) {
	m.Value = int16(wire.ReadBE16(b[0:]))
	// bytes 2-3: MSVC alignment padding (int16 → int32 boundary)
	m.EncodedValue = int32(wire.ReadBE32(b[4:]))
	m.EncoderMul = int16(wire.ReadBE16(b[8:]))
	m.EncoderAdd = int16(wire.ReadBE16(b[10:]))
	m.NumUses = int32(wire.ReadBE32(b[12:]))
}

func (m *BackgammonDiceInfo) Marshal() []byte {
	b := make([]byte, BackgammonDiceInfoSize)
	wire.WriteBE16(b[0:], uint16(m.Value))
	// bytes 2-3: MSVC alignment padding
	wire.WriteBE32(b[4:], uint32(m.EncodedValue))
	wire.WriteBE16(b[8:], uint16(m.EncoderMul))
	wire.WriteBE16(b[10:], uint16(m.EncoderAdd))
	wire.WriteBE32(b[12:], uint32(m.NumUses))
	return b
}

type BackgammonDiceRoll struct {
	Seat int16
	D1   BackgammonDiceInfo
	D2   BackgammonDiceInfo
}

func (m *BackgammonDiceRoll) Marshal() []byte {
	b := make([]byte, BackgammonDiceRollSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	// bytes 2-3: MSVC alignment padding (int16 seat → DICEINFO boundary)
	copy(b[4:20], m.D1.Marshal())
	copy(b[20:36], m.D2.Marshal())
	return b
}

type BackgammonEndLog struct {
	NumPoints    int32
	Reason       int16
	SeatLosing   int16
	SeatQuitting int16
	Rfu          int16
}

func (m *BackgammonEndLog) Unmarshal(b []byte) {
	m.NumPoints = int32(wire.ReadLE32(b[0:]))
	m.Reason = int16(wire.ReadLE16(b[4:]))
	m.SeatLosing = int16(wire.ReadLE16(b[6:]))
	m.SeatQuitting = int16(wire.ReadLE16(b[8:]))
	m.Rfu = int16(wire.ReadLE16(b[10:]))
}

func (m *BackgammonEndLog) Marshal() []byte {
	b := make([]byte, BackgammonEndLogSize)
	wire.WriteLE32(b[0:], uint32(m.NumPoints))
	wire.WriteLE16(b[4:], uint16(m.Reason))
	wire.WriteLE16(b[6:], uint16(m.SeatLosing))
	wire.WriteLE16(b[8:], uint16(m.SeatQuitting))
	wire.WriteLE16(b[10:], uint16(m.Rfu))
	return b
}

type BackgammonFirstMove struct {
	NumPoints int32
	Seat      int16
	Rfu       int16
}

func (m *BackgammonFirstMove) Unmarshal(b []byte) {
	m.NumPoints = int32(wire.ReadBE32(b[0:]))
	m.Seat = int16(wire.ReadLE16(b[4:]))
	m.Rfu = int16(wire.ReadLE16(b[6:]))
}

func (m *BackgammonFirstMove) Marshal() []byte {
	b := make([]byte, BackgammonFirstMoveSize)
	wire.WriteBE32(b[0:], uint32(m.NumPoints))
	wire.WriteLE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	return b
}

type BackgammonEndTurn struct {
	Seat int16
}

func (m *BackgammonEndTurn) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadLE16(b[0:]))
}

func (m *BackgammonEndTurn) Marshal() []byte {
	b := make([]byte, BackgammonEndTurnSize)
	wire.WriteLE16(b[0:], uint16(m.Seat))
	return b
}

type BackgammonTransactionItem struct {
	EntryTag int32
	EntryIdx int32
	EntryVal int32
}

type BackgammonTransaction struct {
	User     uint32
	Seat     int32
	TransCnt int32
	TransTag int32
	Items    []BackgammonTransactionItem
}

func (m *BackgammonTransaction) Unmarshal(b []byte) {
	m.User = wire.ReadLE32(b[0:])
	m.Seat = int32(wire.ReadLE32(b[4:]))
	m.TransCnt = int32(wire.ReadLE32(b[8:]))
	m.TransTag = int32(wire.ReadLE32(b[12:]))
	if m.TransCnt <= 0 {
		return
	}
	m.Items = make([]BackgammonTransactionItem, 0, m.TransCnt)
	off := BackgammonTransactionHdrSize
	for i := int32(0); i < m.TransCnt && off+BackgammonTransactionItemSize <= len(b); i++ {
		m.Items = append(m.Items, BackgammonTransactionItem{
			EntryTag: int32(wire.ReadLE32(b[off+0:])),
			EntryIdx: int32(wire.ReadLE32(b[off+4:])),
			EntryVal: int32(wire.ReadLE32(b[off+8:])),
		})
		off += BackgammonTransactionItemSize
	}
}

func (m *BackgammonTransaction) Marshal() []byte {
	b := make([]byte, BackgammonTransactionHdrSize+len(m.Items)*BackgammonTransactionItemSize)
	wire.WriteLE32(b[0:], m.User)
	wire.WriteLE32(b[4:], uint32(m.Seat))
	wire.WriteLE32(b[8:], uint32(len(m.Items)))
	wire.WriteLE32(b[12:], uint32(m.TransTag))
	off := BackgammonTransactionHdrSize
	for _, item := range m.Items {
		wire.WriteLE32(b[off+0:], uint32(item.EntryTag))
		wire.WriteLE32(b[off+4:], uint32(item.EntryIdx))
		wire.WriteLE32(b[off+8:], uint32(item.EntryVal))
		off += BackgammonTransactionItemSize
	}
	return b
}
