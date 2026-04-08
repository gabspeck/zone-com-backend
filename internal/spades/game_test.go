package spades

import (
	"testing"

	"zone.com/internal/proto"
)

func TestTrickWinnerHighCard(t *testing.T) {
	// Lead diamonds, highest diamond wins
	cards := [NumPlayers]byte{
		0,  // 2 of diamonds (seat 0 leads)
		5,  // 7 of diamonds
		12, // A of diamonds
		3,  // 5 of diamonds
	}
	winner := TrickWinner(cards, 0)
	if winner != 2 {
		t.Errorf("winner = %d, want 2 (ace of diamonds)", winner)
	}
}

func TestTrickWinnerTrump(t *testing.T) {
	// Lead clubs, but seat 2 plays a spade (trump)
	cards := [NumPlayers]byte{
		25, // A of clubs (seat 0 leads)
		14, // 3 of clubs
		39, // 2 of spades (trump!)
		20, // 9 of clubs
	}
	winner := TrickWinner(cards, 0)
	if winner != 2 {
		t.Errorf("winner = %d, want 2 (trump wins)", winner)
	}
}

func TestTrickWinnerHighestTrump(t *testing.T) {
	// Two players play spades, highest wins
	cards := [NumPlayers]byte{
		0,  // 2 of diamonds (lead)
		39, // 2 of spades
		51, // A of spades
		3,  // 5 of diamonds
	}
	winner := TrickWinner(cards, 0)
	if winner != 2 {
		t.Errorf("winner = %d, want 2 (ace of spades)", winner)
	}
}

func TestTrickWinnerOffSuitNoTrump(t *testing.T) {
	// Off-suit without trump doesn't win
	cards := [NumPlayers]byte{
		0,  // 2 of diamonds (lead)
		12, // A of diamonds
		25, // A of clubs (off-suit, no spade)
		3,  // 5 of diamonds
	}
	winner := TrickWinner(cards, 0)
	if winner != 1 {
		t.Errorf("winner = %d, want 1 (ace of diamonds)", winner)
	}
}

func TestValidBid(t *testing.T) {
	for _, b := range []byte{0, 1, 5, 13, BidDoubleNil} {
		if !ValidBid(b) {
			t.Errorf("bid %d should be valid", b)
		}
	}
	for _, b := range []byte{14, 20, 0x7F} {
		if ValidBid(b) {
			t.Errorf("bid %d should be invalid", b)
		}
	}
}

func TestScoreHandMadeBid(t *testing.T) {
	bids := [NumPlayers]byte{4, 3, 3, 3} // team0: 4+3=7, team1: 3+3=6
	tricks := [NumPlayers]int{5, 4, 3, 1} // team0: 5+3=8, team1: 4+1=5
	bags := [NumTeams]int{0, 0}

	score, newBags := ScoreHand(bids, tricks, bags)

	// Team 0: bid 7, got 8 → base=70, bags=1
	if score.Base[0] != 70 {
		t.Errorf("team0 base = %d, want 70", score.Base[0])
	}
	if score.BagBonus[0] != 1 {
		t.Errorf("team0 bagbonus = %d, want 1", score.BagBonus[0])
	}
	if score.Scores[0] != 71 {
		t.Errorf("team0 scores = %d, want 71", score.Scores[0])
	}
	if newBags[0] != 1 {
		t.Errorf("team0 newBags = %d, want 1", newBags[0])
	}

	// Team 1: bid 6, got 5 → set: base=-60
	if score.Base[1] != -60 {
		t.Errorf("team1 base = %d, want -60", score.Base[1])
	}
	if score.Scores[1] != -60 {
		t.Errorf("team1 scores = %d, want -60", score.Scores[1])
	}
}

func TestScoreHandNilSuccess(t *testing.T) {
	bids := [NumPlayers]byte{4, 0, 0, 5} // seat 1: nil, seat 2: nil
	tricks := [NumPlayers]int{6, 0, 0, 7}
	bags := [NumTeams]int{0, 0}

	score, _ := ScoreHand(bids, tricks, bags)

	// Team 0 (seats 0,2): bid effective=4+0=4, tricks=6+0=6 → base=40, bags=2, nil +100 (seat2 success)
	if score.Base[0] != 40 {
		t.Errorf("team0 base = %d, want 40", score.Base[0])
	}
	if score.Nil[0] != 100 {
		t.Errorf("team0 nil = %d, want 100", score.Nil[0])
	}
	// Team 1 (seats 1,3): bid effective=0+5=5, tricks=0+7=7 → base=50, bags=2, nil +100 (seat1 success)
	if score.Nil[1] != 100 {
		t.Errorf("team1 nil = %d, want 100", score.Nil[1])
	}
}

func TestScoreHandNilFail(t *testing.T) {
	bids := [NumPlayers]byte{4, 0, 3, 6} // seat 1: nil
	tricks := [NumPlayers]int{5, 2, 3, 3} // seat 1 took 2 tricks — nil failed
	bags := [NumTeams]int{0, 0}

	score, _ := ScoreHand(bids, tricks, bags)

	// Team 1: nil penalty for seat 1
	if score.Nil[1] != -100 {
		t.Errorf("team1 nil = %d, want -100", score.Nil[1])
	}
}

func TestScoreHandDoubleNil(t *testing.T) {
	bids := [NumPlayers]byte{5, BidDoubleNil, 5, 3}
	tricks := [NumPlayers]int{6, 0, 4, 3}
	bags := [NumTeams]int{0, 0}

	score, _ := ScoreHand(bids, tricks, bags)

	// Team 1 (seats 1,3): seat 1 double nil success → +200
	if score.Nil[1] != 200 {
		t.Errorf("team1 nil = %d, want 200", score.Nil[1])
	}
}

func TestScoreHandBagPenalty(t *testing.T) {
	bids := [NumPlayers]byte{3, 3, 3, 3}
	tricks := [NumPlayers]int{5, 4, 2, 2} // team0: 7 tricks on 6 bid = 1 bag
	bags := [NumTeams]int{9, 0}            // team0 had 9 bags → penalty

	score, newBags := ScoreHand(bids, tricks, bags)

	// Team 0: bags go 9+1=10 → penalty -100, reset to 0
	if score.BagPenalty[0] != -100 {
		t.Errorf("team0 bagpenalty = %d, want -100", score.BagPenalty[0])
	}
	if newBags[0] != 0 {
		t.Errorf("team0 newBags = %d, want 0", newBags[0])
	}
}

func TestDeal(t *testing.T) {
	deck := NewDeck()
	hands := deck.Deal()

	seen := make(map[byte]bool)
	for seat := 0; seat < NumPlayers; seat++ {
		for i := 0; i < CardsPerPlayer; i++ {
			c := hands[seat][i]
			if c >= 52 {
				t.Fatalf("seat %d card %d: invalid card %d", seat, i, c)
			}
			if seen[c] {
				t.Fatalf("duplicate card %d", c)
			}
			seen[c] = true
		}
	}
	if len(seen) != DeckSize {
		t.Errorf("dealt %d unique cards, want %d", len(seen), DeckSize)
	}
}

func TestValidCardToPlay(t *testing.T) {
	h := NewHand([proto.SpadesNumCardsInHand]byte{0, 1, 2, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48})
	// hand has diamonds (0,1,2) and spades (39-48)

	// Leading: can't lead spades if trumps not broken and has non-spades
	if ValidCardToPlay(39, &h, CardNone, true, false) {
		t.Error("should not be able to lead spades when trumps not broken")
	}
	// Leading: can lead non-spades
	if !ValidCardToPlay(0, &h, CardNone, true, false) {
		t.Error("should be able to lead diamonds")
	}
	// Leading: can lead spades if trumps broken
	if !ValidCardToPlay(39, &h, CardNone, true, true) {
		t.Error("should be able to lead spades when broken")
	}

	// Following: must follow suit
	if ValidCardToPlay(39, &h, 5, false, false) {
		t.Error("should not play spade when have diamonds and diamonds led")
	}
	if !ValidCardToPlay(0, &h, 5, false, false) {
		t.Error("should be able to follow suit (diamonds)")
	}

	// Following: can play off-suit if void
	spadesOnly := NewHand([proto.SpadesNumCardsInHand]byte{39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51})
	if !ValidCardToPlay(39, &spadesOnly, 5, false, false) {
		t.Error("should be able to play spade when void in diamonds")
	}
}
