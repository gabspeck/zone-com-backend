package proto

import (
	"unicode/utf16"

	"zone.com/internal/wire"
)

const (
	ReversiNewGameMsgSize    = 20
	ReversiMovePieceSize     = 6
	ReversiEndGameSize       = 8
	ReversiEndLogSize        = 16
	ReversiFinishMoveSize    = 12
	ReversiTalkHdrSize       = 8
	ReversiVoteNewGameSize   = 2
	ReversiGameStateReqSize  = 8
	ReversiMoveTimeoutSize   = 72
	ReversiPlayerInfoSize    = 104
	ReversiGameStateRespSize = 228
)

type ReversiNewGame struct {
	ProtocolSignature int32
	ProtocolVersion   int32
	ClientVersion     int32
	PlayerID          uint32
	Seat              int16
	Rfu               int16
}

func (m *ReversiNewGame) Unmarshal(b []byte) {
	m.ProtocolSignature = int32(wire.ReadBE32(b[0:]))
	m.ProtocolVersion = int32(wire.ReadBE32(b[4:]))
	m.ClientVersion = int32(wire.ReadBE32(b[8:]))
	m.PlayerID = wire.ReadBE32(b[12:])
	m.Seat = int16(wire.ReadBE16(b[16:]))
	m.Rfu = int16(wire.ReadBE16(b[18:]))
}

func (m *ReversiNewGame) Marshal() []byte {
	b := make([]byte, ReversiNewGameMsgSize)
	wire.WriteBE32(b[0:], uint32(m.ProtocolSignature))
	wire.WriteBE32(b[4:], uint32(m.ProtocolVersion))
	wire.WriteBE32(b[8:], uint32(m.ClientVersion))
	wire.WriteBE32(b[12:], m.PlayerID)
	wire.WriteBE16(b[16:], uint16(m.Seat))
	wire.WriteBE16(b[18:], uint16(m.Rfu))
	return b
}

type ReversiMovePiece struct {
	Seat int16
	Rfu  int16
	Col  byte
	Row  byte
}

func (m *ReversiMovePiece) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Rfu = int16(wire.ReadLE16(b[2:]))
	m.Col = b[4]
	m.Row = b[5]
}

func (m *ReversiMovePiece) Marshal() []byte {
	b := make([]byte, ReversiMovePieceSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteLE16(b[2:], uint16(m.Rfu))
	b[4] = m.Col
	b[5] = m.Row
	return b
}

type ReversiEndGame struct {
	Seat  int16
	Rfu   int16
	Flags uint32
}

func (m *ReversiEndGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Rfu = int16(wire.ReadLE16(b[2:]))
	m.Flags = wire.ReadBE32(b[4:])
}

func (m *ReversiEndGame) Marshal() []byte {
	b := make([]byte, ReversiEndGameSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteLE16(b[2:], uint16(m.Rfu))
	wire.WriteBE32(b[4:], m.Flags)
	return b
}

type ReversiEndLog struct {
	NumPoints    int32
	Reason       int16
	SeatLosing   int16
	SeatQuitting int16
	Rfu          int16
	PieceCount   [2]int16
}

func (m *ReversiEndLog) Unmarshal(b []byte) {
	m.NumPoints = int32(wire.ReadLE32(b[0:]))
	m.Reason = int16(wire.ReadLE16(b[4:]))
	m.SeatLosing = int16(wire.ReadLE16(b[6:]))
	m.SeatQuitting = int16(wire.ReadLE16(b[8:]))
	m.Rfu = int16(wire.ReadLE16(b[10:]))
	m.PieceCount[0] = int16(wire.ReadLE16(b[12:]))
	m.PieceCount[1] = int16(wire.ReadLE16(b[14:]))
}

func (m *ReversiEndLog) Marshal() []byte {
	b := make([]byte, ReversiEndLogSize)
	wire.WriteLE32(b[0:], uint32(m.NumPoints))
	wire.WriteLE16(b[4:], uint16(m.Reason))
	wire.WriteLE16(b[6:], uint16(m.SeatLosing))
	wire.WriteLE16(b[8:], uint16(m.SeatQuitting))
	wire.WriteLE16(b[10:], uint16(m.Rfu))
	wire.WriteLE16(b[12:], uint16(m.PieceCount[0]))
	wire.WriteLE16(b[14:], uint16(m.PieceCount[1]))
	return b
}

type ReversiFinishMove struct {
	Seat  int16
	Rfu   int16
	Time  uint32
	Piece byte
}

func (m *ReversiFinishMove) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Rfu = int16(wire.ReadLE16(b[2:]))
	m.Time = wire.ReadBE32(b[4:])
	if len(b) > 8 {
		m.Piece = b[8]
	}
}

func (m *ReversiFinishMove) Marshal() []byte {
	b := make([]byte, ReversiFinishMoveSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteLE16(b[2:], uint16(m.Rfu))
	wire.WriteBE32(b[4:], m.Time)
	b[8] = m.Piece
	return b
}

type ReversiTalk struct {
	UserID     uint32
	Seat       int16
	MessageLen uint16
}

func (m *ReversiTalk) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.MessageLen = wire.ReadBE16(b[6:])
}

func (m *ReversiTalk) Marshal() []byte {
	b := make([]byte, ReversiTalkHdrSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteBE16(b[6:], m.MessageLen)
	return b
}

type ReversiGameStateReq struct {
	UserID uint32
	Seat   int16
	Rfu    int16
}

func (m *ReversiGameStateReq) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.Rfu = int16(wire.ReadLE16(b[6:]))
}

func (m *ReversiGameStateReq) Marshal() []byte {
	b := make([]byte, ReversiGameStateReqSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	return b
}

type ReversiVoteNewGame struct {
	Seat int16
}

func (m *ReversiVoteNewGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
}

func (m *ReversiVoteNewGame) Marshal() []byte {
	b := make([]byte, ReversiVoteNewGameSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	return b
}

type ReversiPlayerInfo struct {
	UserID uint32
	Name   string
	Host   string
}

type ReversiGameStateResp struct {
	UserID      uint32
	Seat        int16
	Rfu         int16
	GameState   int16
	NewGameVote [2]bool
	FinalScore  int16
	WhiteScore  int16
	BlackScore  int16
	Players     [2]ReversiPlayerInfo
	State       []byte
}

func (m *ReversiGameStateResp) Marshal() []byte {
	b := make([]byte, ReversiGameStateRespSize+len(m.State))
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	wire.WriteBE16(b[8:], uint16(m.GameState))
	if m.NewGameVote[0] {
		wire.WriteBE16(b[10:], 1)
	}
	if m.NewGameVote[1] {
		wire.WriteBE16(b[12:], 1)
	}
	wire.WriteBE16(b[14:], uint16(m.FinalScore))
	wire.WriteBE16(b[16:], uint16(m.WhiteScore))
	wire.WriteBE16(b[18:], uint16(m.BlackScore))
	off := 20
	for _, p := range m.Players {
		wire.WriteBE32(b[off+0:], p.UserID)
		writeUTF16Fixed(b[off+4:off+68], p.Name)
		writeUTF16Fixed(b[off+68:off+102], p.Host)
		off += ReversiPlayerInfoSize
	}
	copy(b[ReversiGameStateRespSize:], m.State)
	return b
}

type ReversiMoveTimeout struct {
	UserID   uint32
	Seat     int16
	Timeout  int16
	UserName string
}

func (m *ReversiMoveTimeout) Marshal() []byte {
	b := make([]byte, ReversiMoveTimeoutSize)
	wire.WriteLE32(b[0:], m.UserID)
	wire.WriteLE16(b[4:], uint16(m.Seat))
	wire.WriteLE16(b[6:], uint16(m.Timeout))
	writeUTF16Fixed(b[8:], m.UserName)
	return b
}

func writeUTF16Fixed(dst []byte, s string) {
	u := utf16.Encode([]rune(s))
	maxUnits := len(dst) / 2
	if len(u) >= maxUnits {
		u = u[:maxUnits-1]
	}
	for i, r := range u {
		wire.WriteLE16(dst[i*2:], r)
	}
}
