package checkers

import (
	"fmt"
	"log"
)

// GameState tracks the current phase of a checkers game.
type GameState int

const (
	StateWaitingForPlayers GameState = iota
	StatePlaying
	StateGameOver
)

// Game manages a single checkers game between two players.
type Game struct {
	Board        Board
	PlayerToMove int // PlayerBlack or PlayerWhite
	MoveCount    int
	MoveOver     bool // true when current player's move sequence is complete (no more jumps)
	DrawOfferer  int16
	NewGameVotes [2]bool
	State        GameState
	FinalScore   int16
	LastFlags    uint32
}

// NewGame creates a new checkers game.
func NewGame() *Game {
	return &Game{
		Board:        NewBoard(),
		PlayerToMove: PlayerBlack, // Black moves first
		DrawOfferer:  -1,
		State:        StateWaitingForPlayers,
	}
}

// Reset resets the game for a new round.
func (g *Game) Reset() {
	log.Printf("[game] Reset: clearing board and state for new game")
	g.Board = NewBoard()
	g.PlayerToMove = PlayerBlack
	g.MoveCount = 0
	g.MoveOver = false
	g.DrawOfferer = -1
	g.NewGameVotes = [2]bool{}
	g.State = StatePlaying
	g.FinalScore = 0
	g.LastFlags = 0
	log.Printf("[game] Reset: state=Playing, playerToMove=Black(0)")
}

// HandleMovePiece processes a move from a player.
// Returns the flags from the move result.
func (g *Game) HandleMovePiece(seat int16, startCol, startRow, finishCol, finishRow byte) (uint32, error) {
	log.Printf("[game] HandleMovePiece: seat=%d (%d,%d)->(%d,%d) state=%d playerToMove=%d moveOver=%v",
		seat, startCol, startRow, finishCol, finishRow, g.State, g.PlayerToMove, g.MoveOver)

	if g.State != StatePlaying {
		return 0, fmt.Errorf("game not in playing state (state=%d)", g.State)
	}
	if int(seat) != g.PlayerToMove {
		return 0, fmt.Errorf("not seat %d's turn (current: %d)", seat, g.PlayerToMove)
	}
	if g.MoveOver {
		return 0, fmt.Errorf("move already finished, waiting for FinishMove")
	}

	piece := g.Board[startRow][startCol]
	log.Printf("[game] HandleMovePiece: piece at (%d,%d) = 0x%02x (%s)", startCol, startRow, byte(piece), pieceName(piece))

	move := Move{
		Start:  Square{Col: startCol, Row: startRow},
		Finish: Square{Col: finishCol, Row: finishRow},
	}

	result, ok := ValidateMove(g.Board, g.PlayerToMove, move)
	if !ok {
		log.Printf("[game] HandleMovePiece: ILLEGAL move rejected")
		return 0, fmt.Errorf("illegal move")
	}

	g.Board = result.NewBoard
	g.LastFlags = result.Flags

	if !result.ContinueJump {
		g.MoveOver = true
		log.Printf("[game] HandleMovePiece: move sequence complete (moveOver=true)")
	} else {
		log.Printf("[game] HandleMovePiece: multi-jump in progress (moveOver=false, must continue)")
	}

	if result.Captured != None {
		log.Printf("[game] HandleMovePiece: captured %s", pieceName(result.Captured))
	}

	log.Printf("[game] HandleMovePiece: result flags=0x%x continueJump=%v", result.Flags, result.ContinueJump)
	return result.Flags, nil
}

func pieceName(p Piece) string {
	switch p {
	case None:
		return "none"
	case BlackPawn:
		return "BlackPawn"
	case BlackKing:
		return "BlackKing"
	case WhitePawn:
		return "WhitePawn"
	case WhiteKing:
		return "WhiteKing"
	default:
		return fmt.Sprintf("Unknown(0x%02x)", byte(p))
	}
}

// HandleFinishMove ends the current player's turn.
// Returns stalemate flags if applicable.
func (g *Game) HandleFinishMove(seat int16) (uint32, error) {
	log.Printf("[game] HandleFinishMove: seat=%d state=%d playerToMove=%d moveOver=%v moveCount=%d",
		seat, g.State, g.PlayerToMove, g.MoveOver, g.MoveCount)

	if g.State != StatePlaying {
		return 0, fmt.Errorf("game not in playing state (state=%d)", g.State)
	}
	if int(seat) != g.PlayerToMove {
		return 0, fmt.Errorf("not seat %d's turn (current=%d)", seat, g.PlayerToMove)
	}
	if !g.MoveOver {
		return 0, fmt.Errorf("move not complete (moveOver=false)")
	}

	// Check for stalemate (next player can't move)
	nextPlayer := (g.PlayerToMove + 1) & 1
	stalemate := CheckStalemate(g.Board, nextPlayer)
	if stalemate {
		g.LastFlags |= FlagStalemate
		log.Printf("[game] HandleFinishMove: STALEMATE detected for player %d", nextPlayer)
	}

	flags := g.LastFlags
	g.MoveCount++
	prevPlayer := g.PlayerToMove
	g.PlayerToMove = nextPlayer
	g.MoveOver = false

	log.Printf("[game] HandleFinishMove: turn %d complete (player %d -> %d), flags=0x%x",
		g.MoveCount, prevPlayer, nextPlayer, flags)

	// Check game over
	if over, score := IsGameOver(flags, (g.PlayerToMove+1)&1); over {
		g.State = StateGameOver
		g.FinalScore = int16(score)
		log.Printf("[game] HandleFinishMove: GAME OVER, score=%d", score)
	}

	return flags, nil
}

// HandleEndGame processes a resign or draw-accepted end.
func (g *Game) HandleEndGame(seat int16, flags uint32) error {
	log.Printf("[game] HandleEndGame: seat=%d flags=0x%x state=%d", seat, flags, g.State)

	if g.State != StatePlaying {
		return fmt.Errorf("game not in playing state (state=%d)", g.State)
	}

	g.LastFlags = flags

	// Advance move count to record the ending
	nextPlayer := (g.PlayerToMove + 1) & 1
	g.MoveCount++
	g.PlayerToMove = nextPlayer

	if over, score := IsGameOver(flags, int(seat)); over {
		g.State = StateGameOver
		g.FinalScore = int16(score)
		log.Printf("[game] HandleEndGame: game over, score=%d (flags=0x%x)", score, flags)
	} else {
		g.State = StateGameOver
		log.Printf("[game] HandleEndGame: game ended (no specific score)")
	}
	return nil
}

// HandleDraw processes a draw offer/response.
func (g *Game) HandleDraw(seat, vote int16) {
	log.Printf("[game] HandleDraw: seat=%d vote=%d drawOfferer=%d", seat, vote, g.DrawOfferer)
	if vote == 0 { // Offer draw
		g.DrawOfferer = seat
		log.Printf("[game] HandleDraw: draw offered by seat %d", seat)
		return
	}
	if vote == 1 { // AcceptDraw - handled elsewhere via EndGame
		log.Printf("[game] HandleDraw: draw accepted (will be finalized via EndGame)")
		return
	}
	// RefuseDraw - clear draw state
	log.Printf("[game] HandleDraw: draw refused, clearing draw state")
	g.DrawOfferer = -1
}

// VoteNewGame registers a new game vote from a seat.
// Returns true if both players have voted.
func (g *Game) VoteNewGame(seat int16) bool {
	g.NewGameVotes[seat] = true
	log.Printf("[game] VoteNewGame: seat %d voted, votes=[%v, %v]", seat, g.NewGameVotes[0], g.NewGameVotes[1])
	return g.NewGameVotes[0] && g.NewGameVotes[1]
}
