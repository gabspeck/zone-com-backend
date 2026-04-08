package hearts

import (
	"testing"

	"zone.com/internal/proto"
)

func TestDealDistributes13Each(t *testing.T) {
	d := NewDeck()
	hands := d.Deal()
	for seat := 0; seat < NumPlayers; seat++ {
		count := 0
		for _, c := range hands[seat] {
			if c != CardNone {
				count++
			}
		}
		if count != CardsPerPlayer {
			t.Errorf("seat %d got %d cards, want %d", seat, count, CardsPerPlayer)
		}
	}
}

func TestDealNoDuplicates(t *testing.T) {
	d := NewDeck()
	hands := d.Deal()
	seen := make(map[byte]bool)
	for seat := 0; seat < NumPlayers; seat++ {
		for _, c := range hands[seat] {
			if c == CardNone {
				continue
			}
			if seen[c] {
				t.Fatalf("duplicate card %d in seat %d", c, seat)
			}
			seen[c] = true
		}
	}
	if len(seen) != DeckSize {
		t.Errorf("total cards = %d, want %d", len(seen), DeckSize)
	}
}

func TestFind2CHolder(t *testing.T) {
	d := NewDeck()
	hands := d.Deal()
	holder := Find2CHolder(hands)
	found := false
	for _, c := range hands[holder] {
		if c == Card2C {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("seat %d claimed to hold 2C but doesn't", holder)
	}
}

func TestPassTarget(t *testing.T) {
	tests := []struct {
		from, dir, want int16
	}{
		{0, proto.HeartsPassLeft, 1},
		{1, proto.HeartsPassLeft, 2},
		{3, proto.HeartsPassLeft, 0},
		{0, proto.HeartsPassRight, 3},
		{1, proto.HeartsPassRight, 0},
		{0, proto.HeartsPassAcross, 2},
		{1, proto.HeartsPassAcross, 3},
		{0, proto.HeartsPassHold, 0},
	}
	for _, tc := range tests {
		got := PassTarget(tc.from, tc.dir)
		if got != tc.want {
			t.Errorf("PassTarget(%d, %d) = %d, want %d", tc.from, tc.dir, got, tc.want)
		}
	}
}

func TestTrickWinner(t *testing.T) {
	// Seat 0 leads 5 of clubs (card 3), seat 1 plays 10 of clubs (card 8) — seat 1 wins
	cards := [NumPlayers]byte{3, 8, 15, 28} // 5C, 10C, 3D, 3H
	winner := TrickWinner(cards, 0)
	if winner != 1 {
		t.Errorf("TrickWinner = %d, want 1", winner)
	}
}

func TestTrickWinnerOffSuit(t *testing.T) {
	// Seat 2 leads ace of clubs (card 12), others play off-suit — seat 2 wins
	cards := [NumPlayers]byte{14, 27, 12, 40} // 2D, AS, AC, 2H
	winner := TrickWinner(cards, 2)
	if winner != 2 {
		t.Errorf("TrickWinner = %d, want 2 (off-suit shouldn't win)", winner)
	}
}

func TestScoreHandNormal(t *testing.T) {
	// seat 0 took QS + 3 hearts = 16 pts, seat 1 took 2 hearts = 2 pts, etc.
	tricks := [NumPlayers][]byte{
		{CardQS, 39, 40, 41}, // QS + 3 hearts = 16
		{42, 43},             // 2 hearts = 2
		{44, 45, 46, 47},     // 4 hearts = 4
		{48, 49, 50, 51},     // 4 hearts = 4
	}
	scores, run := ScoreHand(tricks)
	if run != -1 {
		t.Errorf("runPlayer = %d, want -1", run)
	}
	expected := [NumPlayers]int{16, 2, 4, 4}
	if scores != expected {
		t.Errorf("scores = %v, want %v", scores, expected)
	}
}

func TestScoreHandShootTheMoon(t *testing.T) {
	// seat 2 took all 26 points
	tricks := [NumPlayers][]byte{
		{},
		{},
		{CardQS, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51},
		{},
	}
	scores, run := ScoreHand(tricks)
	if run != 2 {
		t.Errorf("runPlayer = %d, want 2", run)
	}
	expected := [NumPlayers]int{26, 26, 0, 26}
	if scores != expected {
		t.Errorf("scores = %v, want %v", scores, expected)
	}
}

func TestHandContainsAndRemove(t *testing.T) {
	var cards [proto.HeartsMaxNumCardsInHand]byte
	for i := range cards {
		cards[i] = CardNone
	}
	cards[0] = 5
	cards[1] = 10
	cards[2] = 36
	h := NewHand(cards)
	if h.Count != 3 {
		t.Fatalf("count = %d, want 3", h.Count)
	}
	if !h.Contains(36) {
		t.Error("should contain QS")
	}
	if h.Contains(0) {
		t.Error("should not contain 2C")
	}
	h.Remove(10)
	if h.Count != 2 {
		t.Errorf("count after remove = %d, want 2", h.Count)
	}
	if h.Contains(10) {
		t.Error("should not contain 10 after removal")
	}
}
