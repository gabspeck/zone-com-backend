package reversi

import "testing"

func TestInitialBoardAndTurn(t *testing.T) {
	g := NewGame()
	g.Reset()
	if g.PlayerToMove() != PlayerBlack {
		t.Fatalf("playerToMove=%d want %d", g.PlayerToMove(), PlayerBlack)
	}
	if g.Current.Board[3][3] != PieceBlack || g.Current.Board[3][4] != PieceWhite ||
		g.Current.Board[4][3] != PieceWhite || g.Current.Board[4][4] != PieceBlack {
		t.Fatalf("unexpected initial board center")
	}
}

func TestLegalMoveFlipsPieces(t *testing.T) {
	g := NewGame()
	g.Reset()
	if err := g.HandleMove(PlayerBlack, 5, 3); err != nil {
		t.Fatalf("HandleMove() error = %v", err)
	}
	if got := g.Current.Board[3][5]; got != PieceBlack {
		t.Fatalf("placed piece = %02x want %02x", got, PieceBlack)
	}
	if got := g.Current.Board[3][4]; got != PieceBlack {
		t.Fatalf("flipped piece = %02x want %02x", got, PieceBlack)
	}
}

func TestIllegalMoveRejected(t *testing.T) {
	g := NewGame()
	g.Reset()
	if err := g.HandleMove(PlayerBlack, 0, 0); err == nil {
		t.Fatal("expected illegal move error")
	}
}

func TestFinishMovePassesTurnOrKeepsPlayerOnForcedPass(t *testing.T) {
	g := &Game{State: StatePlaying}
	g.Current.Player = PlayerBlack
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			g.Current.Board[row][col] = PieceBlack
		}
	}
	g.Current.Board[0][0] = PieceWhite
	g.Current.Board[0][1] = PieceNone
	g.Current.WhiteScore = 1
	g.Current.BlackScore = 62
	g.HandleFinishMove()
	if g.PlayerToMove() != PlayerBlack {
		t.Fatalf("playerToMove=%d want black to move again on forced pass", g.PlayerToMove())
	}
}

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	g := NewGame()
	g.Reset()
	if err := g.HandleMove(PlayerBlack, 5, 3); err != nil {
		t.Fatalf("HandleMove() error = %v", err)
	}
	g.HandleFinishMove()
	buf := g.Serialize()
	got, err := Deserialize(buf)
	if err != nil {
		t.Fatalf("Deserialize() error = %v", err)
	}
	if got.PlayerToMove() != g.PlayerToMove() {
		t.Fatalf("playerToMove=%d want %d", got.PlayerToMove(), g.PlayerToMove())
	}
	if got.WhiteScore() != g.WhiteScore() || got.BlackScore() != g.BlackScore() {
		t.Fatalf("scores mismatch got=(%d,%d) want=(%d,%d)", got.WhiteScore(), got.BlackScore(), g.WhiteScore(), g.BlackScore())
	}
}
