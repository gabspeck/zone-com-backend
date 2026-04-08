package reversi

import (
	"fmt"
)

const (
	PieceNone  byte = 0x00
	PieceWhite byte = 0x81
	PieceBlack byte = 0x01

	PlayerWhite = 0
	PlayerBlack = 1

	ScoreBlackWins = 0
	ScoreWhiteWins = 1
	ScoreDraw      = 2

	FlagWhiteWins uint32 = 0x0001
	FlagBlackWins uint32 = 0x0002
	FlagDraw      uint32 = 0x0004
	FlagResign    uint32 = 0x0010
	FlagTimeLoss  uint32 = 0x0020

	StateWaitingForPlayers = iota
	StatePlaying
	StateGameOver
)

type Square struct {
	Col byte
	Row byte
}

type Move struct {
	Square Square
}

type State struct {
	LastMove                 Move
	Board                    [8][8]byte
	Flags                    uint32
	Player                   byte
	FlipLevel                byte
	DirectionFlippedLastTime [9]uint16
	WhiteScore               int16
	BlackScore               int16
}

type Game struct {
	State          int
	Current        State
	Old            State
	SquaresChanged [12]Square
	NewGameVotes   [2]bool
	FinalScore     int16
}

func NewGame() *Game {
	g := &Game{State: StateWaitingForPlayers}
	g.Reset()
	g.State = StateWaitingForPlayers
	return g
}

func (g *Game) Reset() {
	g.Current = State{}
	g.Current.Player = PlayerBlack
	g.Current.Board[3][3] = PieceBlack
	g.Current.Board[3][4] = PieceWhite
	g.Current.Board[4][3] = PieceWhite
	g.Current.Board[4][4] = PieceBlack
	g.Current.WhiteScore = 2
	g.Current.BlackScore = 2
	g.Old = g.Current
	g.SquaresChanged = [12]Square{}
	g.NewGameVotes = [2]bool{}
	g.FinalScore = 0
	g.State = StatePlaying
}

func (g *Game) VoteNewGame(seat int16) bool {
	g.NewGameVotes[seat] = true
	return g.NewGameVotes[0] && g.NewGameVotes[1]
}

func (g *Game) PlayerToMove() int {
	return int(g.Current.Player)
}

func (g *Game) HandleMove(seat int16, col, row byte) error {
	if g.State != StatePlaying {
		return fmt.Errorf("game not in playing state")
	}
	if int(seat) != int(g.Current.Player) {
		return fmt.Errorf("not seat %d turn", seat)
	}
	if row >= 8 || col >= 8 {
		return fmt.Errorf("move out of range")
	}
	if g.Current.Board[row][col] != PieceNone {
		return fmt.Errorf("occupied square")
	}

	next, ok := applyMove(g.Current, col, row)
	if !ok {
		return fmt.Errorf("illegal move")
	}
	g.Old = g.Current
	g.Current = next
	g.recalculateSquaresChanged()
	return nil
}

func (g *Game) HandleFinishMove() {
	g.Current.Player = byte((int(g.Current.Player) + 1) & 1)
	if !legalMoveExists(g.Current, g.Current.Player) {
		g.Current.Player = byte((int(g.Current.Player) + 1) & 1)
	}
	calculateScores(&g.Current)
	if over, score := isGameOver(g.Current.Flags); over {
		g.State = StateGameOver
		g.FinalScore = int16(score)
	}
}

func (g *Game) HandleEndGame(flags uint32) {
	g.Current.Flags = flags
	g.HandleFinishMove()
}

func (g *Game) WhiteScore() int16 { return g.Current.WhiteScore }
func (g *Game) BlackScore() int16 { return g.Current.BlackScore }

func (g *Game) Serialize() []byte {
	const stateSize = 96
	const totalSize = stateSize*2 + 24
	b := make([]byte, totalSize)
	writeState(b[0:], g.Current, true)
	writeState(b[stateSize:], g.Old, false)
	off := stateSize * 2
	for _, sq := range g.SquaresChanged {
		b[off] = sq.Col
		b[off+1] = sq.Row
		off += 2
	}
	return b
}

func Deserialize(data []byte) (*Game, error) {
	const stateSize = 96
	const totalSize = stateSize*2 + 24
	if len(data) < totalSize {
		return nil, fmt.Errorf("state too short")
	}
	g := &Game{}
	g.Current = readState(data[0:], true)
	g.Old = readState(data[stateSize:], false)
	off := stateSize * 2
	for i := range g.SquaresChanged {
		g.SquaresChanged[i] = Square{Col: data[off], Row: data[off+1]}
		off += 2
	}
	if over, score := isGameOver(g.Current.Flags); over {
		g.State = StateGameOver
		g.FinalScore = int16(score)
	} else {
		g.State = StatePlaying
	}
	return g, nil
}

func writeState(dst []byte, s State, standard bool) {
	dst[0] = s.LastMove.Square.Col
	dst[1] = s.LastMove.Square.Row
	off := 2
	for row := 0; row < 8; row++ {
		copy(dst[off:off+8], s.Board[row][:])
		off += 8
	}
	off = 68
	if standard {
		writeBE32(dst[off:], s.Flags)
	} else {
		writeLE32(dst[off:], s.Flags)
	}
	dst[72] = s.Player
	dst[73] = s.FlipLevel
	off = 74
	for _, v := range s.DirectionFlippedLastTime {
		writeLE16(dst[off:], v)
		off += 2
	}
	if standard {
		writeBE16(dst[92:], uint16(s.WhiteScore))
		writeBE16(dst[94:], uint16(s.BlackScore))
	} else {
		writeLE16(dst[92:], uint16(s.WhiteScore))
		writeLE16(dst[94:], uint16(s.BlackScore))
	}
}

func readState(src []byte, standard bool) State {
	var s State
	s.LastMove.Square.Col = src[0]
	s.LastMove.Square.Row = src[1]
	off := 2
	for row := 0; row < 8; row++ {
		copy(s.Board[row][:], src[off:off+8])
		off += 8
	}
	if standard {
		s.Flags = readBE32(src[68:])
		s.WhiteScore = int16(readBE16(src[92:]))
		s.BlackScore = int16(readBE16(src[94:]))
	} else {
		s.Flags = readLE32(src[68:])
		s.WhiteScore = int16(readLE16(src[92:]))
		s.BlackScore = int16(readLE16(src[94:]))
	}
	s.Player = src[72]
	s.FlipLevel = src[73]
	off = 74
	for i := range s.DirectionFlippedLastTime {
		s.DirectionFlippedLastTime[i] = readLE16(src[off:])
		off += 2
	}
	return s
}

func (g *Game) recalculateSquaresChanged() {
	idx := 0
	for row := byte(0); row < 8; row++ {
		for col := byte(0); col < 8; col++ {
			if g.Old.Board[row][col] != g.Current.Board[row][col] && idx < len(g.SquaresChanged)-1 {
				g.SquaresChanged[idx] = Square{Col: col, Row: row}
				idx++
			}
		}
	}
	for idx < len(g.SquaresChanged) {
		g.SquaresChanged[idx] = Square{Col: 0, Row: 0xFF}
		idx++
	}
}

func applyMove(state State, col, row byte) (State, bool) {
	playerPiece, opponentPiece := piecesForPlayer(state.Player)
	next := state
	next.LastMove = Move{Square: Square{Col: col, Row: row}}
	next.Flags = 0
	next.FlipLevel = 0
	for i := range next.DirectionFlippedLastTime {
		next.DirectionFlippedLastTime[i] = 1
	}
	flippedAny := false
	next.Board[row][col] = playerPiece
	directions := [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}
	for _, d := range directions {
		var captured [8][2]int
		count := 0
		r, c := int(row)+d[0], int(col)+d[1]
		for r >= 0 && r < 8 && c >= 0 && c < 8 && next.Board[r][c] == opponentPiece {
			captured[count] = [2]int{r, c}
			count++
			r += d[0]
			c += d[1]
		}
		if count == 0 || r < 0 || r >= 8 || c < 0 || c >= 8 || next.Board[r][c] != playerPiece {
			continue
		}
		flippedAny = true
		for i := 0; i < count; i++ {
			next.Board[captured[i][0]][captured[i][1]] = playerPiece
		}
	}
	if !flippedAny {
		return state, false
	}
	calculateScores(&next)
	return next, true
}

func legalMoveExists(state State, player byte) bool {
	test := state
	test.Player = player
	for row := byte(0); row < 8; row++ {
		for col := byte(0); col < 8; col++ {
			if test.Board[row][col] != PieceNone {
				continue
			}
			if _, ok := applyMove(test, col, row); ok {
				return true
			}
		}
	}
	return false
}

func calculateScores(state *State) {
	var white, black int16
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			switch state.Board[row][col] {
			case PieceWhite:
				white++
			case PieceBlack:
				black++
			}
		}
	}
	if !legalMoveExists(*state, PlayerWhite) && !legalMoveExists(*state, PlayerBlack) {
		switch {
		case white > black:
			state.Flags |= FlagWhiteWins
		case black > white:
			state.Flags |= FlagBlackWins
		default:
			state.Flags |= FlagDraw
		}
	}
	state.WhiteScore = white
	state.BlackScore = black
}

func isGameOver(flags uint32) (bool, int) {
	switch {
	case flags&FlagWhiteWins != 0:
		return true, ScoreWhiteWins
	case flags&FlagBlackWins != 0:
		return true, ScoreBlackWins
	case flags&FlagDraw != 0:
		return true, ScoreDraw
	case flags&FlagResign != 0:
		if flags&FlagTimeLoss != 0 {
			return true, ScoreBlackWins
		}
		return true, ScoreWhiteWins
	default:
		return false, 0
	}
}

func piecesForPlayer(player byte) (byte, byte) {
	if player == PlayerWhite {
		return PieceWhite, PieceBlack
	}
	return PieceBlack, PieceWhite
}

func writeBE16(dst []byte, v uint16) { dst[0], dst[1] = byte(v>>8), byte(v) }
func writeLE16(dst []byte, v uint16) { dst[0], dst[1] = byte(v), byte(v>>8) }
func writeBE32(dst []byte, v uint32) {
	dst[0], dst[1], dst[2], dst[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
}
func writeLE32(dst []byte, v uint32) {
	dst[0], dst[1], dst[2], dst[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
func readBE16(src []byte) uint16 { return uint16(src[0])<<8 | uint16(src[1]) }
func readLE16(src []byte) uint16 { return uint16(src[0]) | uint16(src[1])<<8 }
func readBE32(src []byte) uint32 {
	return uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])
}
func readLE32(src []byte) uint32 {
	return uint32(src[0]) | uint32(src[1])<<8 | uint32(src[2])<<16 | uint32(src[3])<<24
}
