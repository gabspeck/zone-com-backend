package proto

import "zone.com/internal/wire"

// Checkers message sizes
const (
	CheckersNewGameSize       = 16 // protSig(4)+protVer(4)+clientVer(4)+playerID(4)+seat(2)+rfu(2) = 18 -- wait
	// Actually: int32(4)+int32(4)+int32(4)+ZUserID(4)+ZSeat(2)+int16(2) = 20
	CheckersNewGameMsgSize    = 20
	CheckersMovePieceSize     = 8  // seat(2)+rfu(2)+move(4)
	CheckersFinishMoveSize    = 12 // seat(2)+drawSeat(2)+time(4)+piece(1)+pad(3) -- actually piece is ZCheckersPiece=BYTE
	// With pack(4): seat(2)+drawSeat(2)+time(4)+piece(1)+pad(3) = 12
	CheckersFinishMoveMsgSize = 12
	CheckersEndGameSize       = 8  // seat(2)+rfu(2)+flags(4)
	CheckersEndLogSize        = 8  // reason(2)+seatLosing(2)+seatQuitting(2)+rfu(2)
	CheckersDrawSize          = 4  // seat(2)+vote(2)
	CheckersTalkHdrSize       = 8  // userID(4)+seat(2)+messageLen(2)
	CheckersVoteNewGameSize   = 2  // seat(2)
	CheckersGameStateReqSize  = 8  // userID(4)+seat(2)+rfu(2)
)

// CheckersNewGame is the check-in / new game message.
// Fields are big-endian per ZCheckersMsgNewGameEndian.
type CheckersNewGame struct {
	ProtocolSignature int32
	ProtocolVersion   int32
	ClientVersion     int32
	PlayerID          uint32
	Seat              int16
	Rfu               int16
}

func (m *CheckersNewGame) Unmarshal(b []byte) {
	m.ProtocolSignature = int32(wire.ReadBE32(b[0:]))
	m.ProtocolVersion = int32(wire.ReadBE32(b[4:]))
	m.ClientVersion = int32(wire.ReadBE32(b[8:]))
	m.PlayerID = wire.ReadBE32(b[12:])
	m.Seat = int16(wire.ReadBE16(b[16:]))
	m.Rfu = int16(wire.ReadBE16(b[18:]))
}

func (m *CheckersNewGame) Marshal() []byte {
	b := make([]byte, CheckersNewGameMsgSize)
	wire.WriteBE32(b[0:], uint32(m.ProtocolSignature))
	wire.WriteBE32(b[4:], uint32(m.ProtocolVersion))
	wire.WriteBE32(b[8:], uint32(m.ClientVersion))
	wire.WriteBE32(b[12:], m.PlayerID)
	wire.WriteBE16(b[16:], uint16(m.Seat))
	wire.WriteBE16(b[18:], uint16(m.Rfu))
	return b
}

// CheckersMovePiece is a move message.
// seat is BE per ZCheckersMsgMovePieceEndian. Move bytes are raw.
type CheckersMovePiece struct {
	Seat      int16
	Rfu       int16
	StartCol  byte
	StartRow  byte
	FinishCol byte
	FinishRow byte
}

func (m *CheckersMovePiece) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Rfu = int16(wire.ReadBE16(b[2:])) // not swapped in C, but we read raw anyway
	m.StartCol = b[4]
	m.StartRow = b[5]
	m.FinishCol = b[6]
	m.FinishRow = b[7]
}

func (m *CheckersMovePiece) Marshal() []byte {
	b := make([]byte, CheckersMovePieceSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.Rfu))
	b[4] = m.StartCol
	b[5] = m.StartRow
	b[6] = m.FinishCol
	b[7] = m.FinishRow
	return b
}

// CheckersFinishMove confirms the end of a player's turn.
// Fields are BE per ZCheckersMsgFinishMoveEndian.
type CheckersFinishMove struct {
	Seat     int16
	DrawSeat int16
	Time     uint32
	Piece    byte
}

func (m *CheckersFinishMove) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.DrawSeat = int16(wire.ReadBE16(b[2:]))
	m.Time = wire.ReadBE32(b[4:])
	if len(b) > 8 {
		m.Piece = b[8]
	}
}

func (m *CheckersFinishMove) Marshal() []byte {
	b := make([]byte, CheckersFinishMoveMsgSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.DrawSeat))
	wire.WriteBE32(b[4:], m.Time)
	if len(b) > 8 {
		b[8] = m.Piece
	}
	return b
}

// CheckersEndGame is resign/draw end message.
type CheckersEndGame struct {
	Seat  int16
	Rfu   int16
	Flags uint32
}

func (m *CheckersEndGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Rfu = int16(wire.ReadBE16(b[2:]))
	m.Flags = wire.ReadBE32(b[4:])
}

func (m *CheckersEndGame) Marshal() []byte {
	b := make([]byte, CheckersEndGameSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.Rfu))
	wire.WriteBE32(b[4:], m.Flags)
	return b
}

// CheckersEndLog is game completion log.
// ZCheckersMsgEndLogEndian is EMPTY in C source - fields are NOT swapped.
type CheckersEndLog struct {
	Reason       int16
	SeatLosing   int16
	SeatQuitting int16
	Rfu          int16
}

func (m *CheckersEndLog) Unmarshal(b []byte) {
	m.Reason = int16(wire.ReadLE16(b[0:]))
	m.SeatLosing = int16(wire.ReadLE16(b[2:]))
	m.SeatQuitting = int16(wire.ReadLE16(b[4:]))
	m.Rfu = int16(wire.ReadLE16(b[6:]))
}

func (m *CheckersEndLog) Marshal() []byte {
	b := make([]byte, CheckersEndLogSize)
	wire.WriteLE16(b[0:], uint16(m.Reason))
	wire.WriteLE16(b[2:], uint16(m.SeatLosing))
	wire.WriteLE16(b[4:], uint16(m.SeatQuitting))
	wire.WriteLE16(b[6:], uint16(m.Rfu))
	return b
}

// CheckersDraw is a draw offer/response.
type CheckersDraw struct {
	Seat int16
	Vote int16
}

func (m *CheckersDraw) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
	m.Vote = int16(wire.ReadBE16(b[2:]))
}

func (m *CheckersDraw) Marshal() []byte {
	b := make([]byte, CheckersDrawSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	wire.WriteBE16(b[2:], uint16(m.Vote))
	return b
}

// CheckersTalk is a chat message (header only; text follows).
type CheckersTalk struct {
	UserID     uint32
	Seat       int16
	MessageLen uint16
}

func (m *CheckersTalk) Unmarshal(b []byte) {
	m.UserID = wire.ReadBE32(b[0:])
	m.Seat = int16(wire.ReadBE16(b[4:]))
	m.MessageLen = wire.ReadBE16(b[6:])
}

func (m *CheckersTalk) Marshal() []byte {
	b := make([]byte, CheckersTalkHdrSize)
	wire.WriteBE32(b[0:], m.UserID)
	wire.WriteBE16(b[4:], uint16(m.Seat))
	wire.WriteBE16(b[6:], m.MessageLen)
	return b
}

// CheckersVoteNewGame is a new game vote.
type CheckersVoteNewGame struct {
	Seat int16
}

func (m *CheckersVoteNewGame) Unmarshal(b []byte) {
	m.Seat = int16(wire.ReadBE16(b[0:]))
}

func (m *CheckersVoteNewGame) Marshal() []byte {
	b := make([]byte, CheckersVoteNewGameSize)
	wire.WriteBE16(b[0:], uint16(m.Seat))
	return b
}
