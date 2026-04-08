package hearts

import (
	"math/rand"

	"zone.com/internal/proto"
)

const (
	NumPlayers     = 4
	CardsPerPlayer = 13
	CardsToPass    = 3
	DeckSize       = 52
	PointsForGame  = 100

	Card2C   = 0
	CardQS   = 36
	CardNone = 127

	SuitClubs    = 0
	SuitDiamonds = 1
	SuitSpades   = 2
	SuitHearts   = 3
)

// PassDirs is the cycling order of pass directions.
var PassDirs = []int16{
	proto.HeartsPassLeft,
	proto.HeartsPassAcross,
	proto.HeartsPassRight,
	proto.HeartsPassHold,
}

func Suit(card byte) int  { return int(card) / 13 }
func Rank(card byte) int  { return int(card) % 13 }
func IsHeart(c byte) bool { return Suit(c) == SuitHearts }
func IsQS(c byte) bool    { return c == CardQS }

// IsPointCard returns true if the card scores points.
func IsPointCard(c byte) bool { return IsHeart(c) || IsQS(c) }

// CardPoints returns the point value of a card (standard rules).
func CardPoints(c byte) int {
	if IsQS(c) {
		return 13
	}
	if IsHeart(c) {
		return 1
	}
	return 0
}

// Deck represents a shuffled 52-card deck.
type Deck [DeckSize]byte

// NewDeck creates and shuffles a deck.
func NewDeck() Deck {
	var d Deck
	for i := range d {
		d[i] = byte(i)
	}
	rand.Shuffle(DeckSize, func(i, j int) { d[i], d[j] = d[j], d[i] })
	return d
}

// Deal distributes 13 cards to each of 4 players.
func (d *Deck) Deal() [NumPlayers][proto.HeartsMaxNumCardsInHand]byte {
	var hands [NumPlayers][proto.HeartsMaxNumCardsInHand]byte
	for i := range hands {
		for j := range hands[i] {
			hands[i][j] = CardNone
		}
	}
	for i := 0; i < DeckSize; i++ {
		hands[i%NumPlayers][i/NumPlayers] = d[i]
	}
	return hands
}

// Find2CHolder returns the seat holding the 2 of Clubs.
func Find2CHolder(hands [NumPlayers][proto.HeartsMaxNumCardsInHand]byte) int16 {
	for seat := 0; seat < NumPlayers; seat++ {
		for _, c := range hands[seat] {
			if c == Card2C {
				return int16(seat)
			}
		}
	}
	return 0
}

// PassTarget returns the seat that `from` passes cards to.
func PassTarget(from int16, dir int16) int16 {
	switch dir {
	case proto.HeartsPassLeft:
		return (from + 1) % NumPlayers
	case proto.HeartsPassAcross:
		return (from + 2) % NumPlayers
	case proto.HeartsPassRight:
		return (from + 3) % NumPlayers
	default:
		return from // hold
	}
}

// TrickWinner returns the seat that wins the trick.
func TrickWinner(cardsPlayed [NumPlayers]byte, leadPlayer int16) int16 {
	ledSuit := Suit(cardsPlayed[leadPlayer])
	winner := leadPlayer
	highRank := Rank(cardsPlayed[leadPlayer])
	for i := 1; i < NumPlayers; i++ {
		seat := (int(leadPlayer) + i) % NumPlayers
		if Suit(cardsPlayed[seat]) == ledSuit && Rank(cardsPlayed[seat]) > highRank {
			highRank = Rank(cardsPlayed[seat])
			winner = int16(seat)
		}
	}
	return winner
}

// ScoreHand calculates per-player scores for a hand.
// Returns scores and the seat that shot the moon (-1 if nobody).
func ScoreHand(trickCards [NumPlayers][]byte) (scores [NumPlayers]int, runPlayer int16) {
	for seat := 0; seat < NumPlayers; seat++ {
		for _, c := range trickCards[seat] {
			scores[seat] += CardPoints(c)
		}
	}
	// Check for shooting the moon
	runPlayer = -1
	for seat := 0; seat < NumPlayers; seat++ {
		if scores[seat] == 26 {
			runPlayer = int16(seat)
			for i := 0; i < NumPlayers; i++ {
				if i == seat {
					scores[i] = 0
				} else {
					scores[i] = 26
				}
			}
			break
		}
	}
	return
}

// Hand tracks the state of cards held by a player.
type Hand struct {
	Cards [proto.HeartsMaxNumCardsInHand]byte
	Count int
}

// NewHand creates a hand from the dealt cards array.
func NewHand(cards [proto.HeartsMaxNumCardsInHand]byte) Hand {
	h := Hand{Cards: cards}
	for _, c := range cards {
		if c != CardNone {
			h.Count++
		}
	}
	return h
}

// Contains returns true if the hand holds the given card.
func (h *Hand) Contains(card byte) bool {
	for _, c := range h.Cards[:h.Count] {
		if c == card {
			return true
		}
	}
	return false
}

// Remove removes a card from the hand.
func (h *Hand) Remove(card byte) bool {
	for i := 0; i < h.Count; i++ {
		if h.Cards[i] == card {
			h.Cards[i] = h.Cards[h.Count-1]
			h.Cards[h.Count-1] = CardNone
			h.Count--
			return true
		}
	}
	return false
}

// Add adds a card to the hand.
func (h *Hand) Add(card byte) {
	h.Cards[h.Count] = card
	h.Count++
}

// HasSuit returns true if the hand contains any card of the given suit.
func (h *Hand) HasSuit(suit int) bool {
	for i := 0; i < h.Count; i++ {
		if Suit(h.Cards[i]) == suit {
			return true
		}
	}
	return false
}

// AllPoints returns true if all cards in hand are point cards.
func (h *Hand) AllPoints() bool {
	for i := 0; i < h.Count; i++ {
		if !IsPointCard(h.Cards[i]) {
			return false
		}
	}
	return h.Count > 0
}
