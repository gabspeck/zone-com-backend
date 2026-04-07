package checkers

import "testing"

func TestNewBoardLayout(t *testing.T) {
	b := NewBoard()
	// Row 0: B _ B _ B _ B _
	if b[0][0] != BlackPawn {
		t.Errorf("b[0][0]: got %d, want %d", b[0][0], BlackPawn)
	}
	if b[0][1] != None {
		t.Errorf("b[0][1]: got %d, want 0", b[0][1])
	}
	// Row 5: _ W _ W _ W _ W
	if b[5][1] != WhitePawn {
		t.Errorf("b[5][1]: got %d, want %d", b[5][1], WhitePawn)
	}
	if b[5][0] != None {
		t.Errorf("b[5][0]: got %d, want 0", b[5][0])
	}
	// Row 3 and 4: empty
	for col := 0; col < 8; col++ {
		if b[3][col] != None {
			t.Errorf("b[3][%d]: not empty", col)
		}
		if b[4][col] != None {
			t.Errorf("b[4][%d]: not empty", col)
		}
	}
}

func TestBlackPawnSimpleMove(t *testing.T) {
	b := NewBoard()
	// Black pawn at (2,2) moves to (3,3)
	move := Move{Start: Square{2, 2}, Finish: Square{3, 3}}
	result, ok := ValidateMove(b, PlayerBlack, move)
	if !ok {
		t.Fatal("valid move rejected")
	}
	if result.Captured != None {
		t.Error("unexpected capture")
	}
	if result.NewBoard[2][2] != None {
		t.Error("start square not cleared")
	}
	if result.NewBoard[3][3] != BlackPawn {
		t.Error("piece not at destination")
	}
}

func TestBlackPawnCantMoveBackward(t *testing.T) {
	var b Board
	b[3][3] = BlackPawn
	move := Move{Start: Square{3, 3}, Finish: Square{2, 2}}
	_, ok := ValidateMove(b, PlayerBlack, move)
	if ok {
		t.Fatal("backward move should be rejected")
	}
}

func TestWhitePawnSimpleMove(t *testing.T) {
	var b Board
	b[5][1] = WhitePawn
	move := Move{Start: Square{1, 5}, Finish: Square{0, 4}}
	result, ok := ValidateMove(b, PlayerWhite, move)
	if !ok {
		t.Fatal("valid move rejected")
	}
	if result.NewBoard[4][0] != WhitePawn {
		t.Error("piece not at destination")
	}
}

func TestJumpCapture(t *testing.T) {
	var b Board
	b[2][2] = BlackPawn
	b[3][3] = WhitePawn // opponent piece to jump over
	move := Move{Start: Square{2, 2}, Finish: Square{4, 4}}
	result, ok := ValidateMove(b, PlayerBlack, move)
	if !ok {
		t.Fatal("jump rejected")
	}
	if result.Captured != WhitePawn {
		t.Errorf("expected capture WhitePawn, got %d", result.Captured)
	}
	if result.NewBoard[3][3] != None {
		t.Error("jumped piece not removed")
	}
	if result.NewBoard[4][4] != BlackPawn {
		t.Error("piece not at destination")
	}
}

func TestMandatoryJump(t *testing.T) {
	var b Board
	b[2][2] = BlackPawn
	b[3][3] = WhitePawn // can jump
	// Try a non-jump move instead
	b[2][4] = BlackPawn
	move := Move{Start: Square{4, 2}, Finish: Square{5, 3}}
	_, ok := ValidateMove(b, PlayerBlack, move)
	if ok {
		t.Fatal("non-jump move should be rejected when jump available")
	}
}

func TestPromotion(t *testing.T) {
	var b Board
	b[6][0] = BlackPawn
	move := Move{Start: Square{0, 6}, Finish: Square{1, 7}}
	result, ok := ValidateMove(b, PlayerBlack, move)
	if !ok {
		t.Fatal("promotion move rejected")
	}
	if result.NewBoard[7][1] != BlackKing {
		t.Errorf("expected BlackKing at dest, got %d", result.NewBoard[7][1])
	}
	if (result.Flags & FlagPromote) == 0 {
		t.Error("FlagPromote not set")
	}
}

func TestKingMovesAnyDirection(t *testing.T) {
	var b Board
	b[4][4] = BlackKing
	// Forward
	_, ok1 := ValidateMove(b, PlayerBlack, Move{Start: Square{4, 4}, Finish: Square{5, 5}})
	// Backward
	_, ok2 := ValidateMove(b, PlayerBlack, Move{Start: Square{4, 4}, Finish: Square{3, 3}})
	if !ok1 {
		t.Error("king forward rejected")
	}
	if !ok2 {
		t.Error("king backward rejected")
	}
}

func TestMultiJumpContinue(t *testing.T) {
	var b Board
	b[0][0] = BlackPawn
	b[1][1] = WhitePawn
	b[3][3] = WhitePawn
	move := Move{Start: Square{0, 0}, Finish: Square{2, 2}}
	result, ok := ValidateMove(b, PlayerBlack, move)
	if !ok {
		t.Fatal("first jump rejected")
	}
	if !result.ContinueJump {
		t.Error("expected ContinueJump")
	}
}

func TestPromotionStopsMultiJump(t *testing.T) {
	var b Board
	b[5][1] = BlackPawn
	b[6][2] = WhitePawn
	// After jumping to (7,3), promotion should stop any further jumps
	move := Move{Start: Square{1, 5}, Finish: Square{3, 7}}
	result, ok := ValidateMove(b, PlayerBlack, move)
	if !ok {
		t.Fatal("jump-to-promotion rejected")
	}
	if result.ContinueJump {
		t.Error("promotion should stop multi-jump")
	}
	if (result.Flags & FlagPromote) == 0 {
		t.Error("FlagPromote not set")
	}
}

func TestStalemate(t *testing.T) {
	var b Board
	// White pawn trapped: can only move forward-diag but both squares blocked
	b[0][0] = WhitePawn
	b[1][1] = BlackPawn // blocks (0,0)->(1,1) -- but wait, white moves toward row 0, and row 0 is already there
	// White pawn at row 0 can't move forward (toward row -1). It's already at the back.
	// Actually white pawns move toward row 0, so at row 0 they'd be promoted already.
	// Let's use a simpler setup: white pawn at row 1 with both forward diagonals blocked by own pieces.
	b = Board{}
	b[1][1] = WhitePawn
	b[0][0] = WhitePawn // own piece blocks forward-left
	b[0][2] = WhitePawn // own piece blocks forward-right
	stale := CheckStalemate(b, PlayerWhite)
	if !stale {
		t.Error("expected stalemate for white")
	}
}

func TestPlayerCanJump(t *testing.T) {
	var b Board
	b[2][2] = BlackPawn
	b[3][3] = WhitePawn
	can, _ := PlayerCanJump(b, PlayerBlack)
	if !can {
		t.Error("expected jump available")
	}
}

func TestGameOverResign(t *testing.T) {
	over, score := IsGameOver(FlagResign, PlayerBlack)
	if !over {
		t.Error("expected game over")
	}
	if score != ScoreWhiteWins {
		t.Errorf("expected white wins, got %d", score)
	}
}

func TestGameOverDraw(t *testing.T) {
	over, score := IsGameOver(FlagDraw, PlayerBlack)
	if !over {
		t.Error("expected game over")
	}
	if score != ScoreDraw {
		t.Errorf("expected draw, got %d", score)
	}
}
