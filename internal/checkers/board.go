package checkers

import "fmt"

// Piece types matching checklib.h
type Piece byte

const (
	None      Piece = 0
	BlackPawn Piece = 1  // zCheckersPieceBlackPawn
	BlackKing Piece = 2  // zCheckersPieceBlackKing
	WhitePawn Piece = 0x81 // zCheckersPieceWhitePawn
	WhiteKing Piece = 0x82 // zCheckersPieceWhiteKing
)

const (
	PlayerBlack = 0 // zCheckersPlayerBlack - moves toward row 7
	PlayerWhite = 1 // zCheckersPlayerWhite - moves toward row 0
)

// Flags matching checklib.h
const (
	FlagStalemate    uint32 = 0x0004
	FlagResign       uint32 = 0x0010
	FlagTimeLoss     uint32 = 0x0020
	FlagWasJump      uint32 = 0x0040
	FlagContinueJump uint32 = 0x0080
	FlagPromote      uint32 = 0x0100
	FlagDraw         uint32 = 0x0200
)

// Score results
const (
	ScoreBlackWins = 0
	ScoreWhiteWins = 1
	ScoreDraw      = 2
)

type Square struct{ Col, Row byte }
type Move struct{ Start, Finish Square }
type Board [8][8]Piece

func PieceColor(p Piece) Piece { return p & 0x80 }
func PieceType(p Piece) Piece  { return p & 0x7f }
func PieceOwner(p Piece) int {
	if PieceColor(p) == 0x80 {
		return PlayerWhite
	}
	return PlayerBlack
}

// NewBoard returns the standard initial checkers layout.
// Matches gBoardStart in checklib.cpp.
// Board[row][col] - row 0-2 = black, rows 5-7 = white.
func NewBoard() Board {
	var b Board
	// Black pawns: rows 0-2
	for row := 0; row < 3; row++ {
		for col := 0; col < 8; col++ {
			if (row+col)%2 == 0 {
				b[row][col] = BlackPawn
			}
		}
	}
	// White pawns: rows 5-7
	for row := 5; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if (row+col)%2 == 0 {
				b[row][col] = WhitePawn
			}
		}
	}
	return b
}

func (b Board) At(sq Square) Piece {
	return b[sq.Row][sq.Col]
}

// String returns a human-readable board representation.
// b=black pawn, B=black king, w=white pawn, W=white king, .=empty
// Rows printed 7 (top) to 0 (bottom) matching visual orientation.
func (b Board) String() string {
	var s string
	s += "    a b c d e f g h\n"
	s += "  +-----------------+\n"
	for row := 7; row >= 0; row-- {
		s += fmt.Sprintf("%d | ", row+1)
		for col := 0; col < 8; col++ {
			p := b[row][col]
			var ch byte
			switch p {
			case None:
				ch = '.'
			case BlackPawn:
				ch = 'b'
			case BlackKing:
				ch = 'B'
			case WhitePawn:
				ch = 'w'
			case WhiteKing:
				ch = 'W'
			default:
				ch = '?'
			}
			s += string(ch)
			if col < 7 {
				s += " "
			}
		}
		s += fmt.Sprintf(" | %d\n", row+1)
	}
	s += "  +-----------------+\n"
	s += "    a b c d e f g h"
	return s
}
