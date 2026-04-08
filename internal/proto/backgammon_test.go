package proto

import "testing"

func TestBackgammonTransactionRoundTrip(t *testing.T) {
	in := BackgammonTransaction{
		User:     123,
		Seat:     1,
		TransTag: 4,
		Items: []BackgammonTransactionItem{
			{EntryTag: 22, EntryIdx: 0, EntryVal: 5},
			{EntryTag: 19, EntryIdx: 2, EntryVal: 6},
		},
	}
	raw := in.Marshal()
	var out BackgammonTransaction
	out.Unmarshal(raw)
	if out.User != in.User || out.Seat != in.Seat || out.TransTag != in.TransTag {
		t.Fatalf("header mismatch: got %+v want %+v", out, in)
	}
	if len(out.Items) != len(in.Items) {
		t.Fatalf("items length mismatch: got %d want %d", len(out.Items), len(in.Items))
	}
	for i := range in.Items {
		if out.Items[i] != in.Items[i] {
			t.Fatalf("item[%d] mismatch: got %+v want %+v", i, out.Items[i], in.Items[i])
		}
	}
}
