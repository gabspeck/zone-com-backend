package proto

import "testing"

func TestCheckersNewGameRoundTrip(t *testing.T) {
	orig := CheckersNewGame{
		ProtocolSignature: int32(CheckersSig),
		ProtocolVersion:   int32(CheckersVersion),
		ClientVersion:     100,
		PlayerID:          42,
		Seat:              1,
	}
	b := orig.Marshal()
	if len(b) != CheckersNewGameMsgSize {
		t.Fatalf("size: got %d, want %d", len(b), CheckersNewGameMsgSize)
	}
	var got CheckersNewGame
	got.Unmarshal(b)
	if got != orig {
		t.Fatalf("round-trip failed:\ngot  %+v\nwant %+v", got, orig)
	}
}

func TestCheckersMovePieceRoundTrip(t *testing.T) {
	orig := CheckersMovePiece{
		Seat:      0,
		StartCol:  2,
		StartRow:  2,
		FinishCol: 3,
		FinishRow: 3,
	}
	b := orig.Marshal()
	var got CheckersMovePiece
	got.Unmarshal(b)
	if got.Seat != orig.Seat || got.StartCol != orig.StartCol ||
		got.StartRow != orig.StartRow || got.FinishCol != orig.FinishCol ||
		got.FinishRow != orig.FinishRow {
		t.Fatalf("round-trip failed:\ngot  %+v\nwant %+v", got, orig)
	}
}

func TestCheckersFinishMoveRoundTrip(t *testing.T) {
	orig := CheckersFinishMove{
		Seat:     0,
		DrawSeat: -1,
		Time:     0,
		Piece:    0,
	}
	b := orig.Marshal()
	if len(b) != CheckersFinishMoveMsgSize {
		t.Fatalf("size: got %d, want %d", len(b), CheckersFinishMoveMsgSize)
	}
	var got CheckersFinishMove
	got.Unmarshal(b)
	if got.Seat != orig.Seat || got.DrawSeat != orig.DrawSeat {
		t.Fatalf("round-trip failed:\ngot  %+v\nwant %+v", got, orig)
	}
}

func TestCheckersDrawRoundTrip(t *testing.T) {
	orig := CheckersDraw{Seat: 1, Vote: AcceptDraw}
	b := orig.Marshal()
	var got CheckersDraw
	got.Unmarshal(b)
	if got != orig {
		t.Fatalf("round-trip: got %+v, want %+v", got, orig)
	}
}
