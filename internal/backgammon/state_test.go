package backgammon

import "testing"

func TestEncodeDiceRoundTripAndUses(t *testing.T) {
	for i := int16(1); i <= 6; i++ {
		d := EncodeDice(i)
		if d.Value != i {
			t.Fatalf("value mismatch: got %d want %d", d.Value, i)
		}
		decoded := ((((d.EncodedValue - 47) / 384) - int32(d.EncoderAdd)) / int32(d.EncoderMul))
		if int16(decoded) != i {
			t.Fatalf("decoded mismatch: got %d want %d", decoded, i)
		}
		EncodeUses(&d, 2)
		uses := (((d.NumUses - int32(d.EncoderAdd+4)) / int32(d.EncoderMul+3)) - 31) / 16
		if uses != 2 {
			t.Fatalf("uses mismatch: got %d want 2", uses)
		}
	}
}

func TestSharedStateDumpSize(t *testing.T) {
	s := NewSharedState()
	dump := s.Dump()
	if len(dump) != s.Size() {
		t.Fatalf("dump size mismatch: got %d want %d", len(dump), s.Size())
	}
	if got := s.Get(StateTagCubeValue, 0); got != 1 {
		t.Fatalf("cube value mismatch: got %d want 1", got)
	}
	if got := s.Get(StateTagTargetScore, 0); got != 3 {
		t.Fatalf("target score mismatch: got %d want 3", got)
	}
}
