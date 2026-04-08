package spades

import (
	"math/rand"

	"zone.com/internal/proto"
)

const (
	NumPlayers     = 4
	NumTeams       = 2
	CardsPerPlayer = 13
	DeckSize       = 52
	PointsForGame  = 500
	MinPoints      = -200

	CardNone = 127

	SuitDiamonds = 0
	SuitClubs    = 1
	SuitHearts   = 2
	SuitSpades   = 3

	BidNone      byte = 0xFF
	BidDoubleNil byte = 0x80
)

func Suit(card byte) int { return int(card) / 13 }
func Rank(card byte) int { return int(card) % 13 }
func Team(seat int16) int { return int(seat) % NumTeams }
func Partner(seat int16) int16 { return (seat + 2) % NumPlayers }

type Deck [DeckSize]byte

func NewDeck() Deck {
	var d Deck
	for i := range d {
		d[i] = byte(i)
	}
	rand.Shuffle(DeckSize, func(i, j int) { d[i], d[j] = d[j], d[i] })
	return d
}

func (d *Deck) Deal() [NumPlayers][proto.SpadesNumCardsInHand]byte {
	var hands [NumPlayers][proto.SpadesNumCardsInHand]byte
	for i := 0; i < DeckSize; i++ {
		hands[i%NumPlayers][i/NumPlayers] = d[i]
	}
	return hands
}

// TrickWinner returns the seat that wins the trick. Spades are trump.
func TrickWinner(cardsPlayed [NumPlayers]byte, leadPlayer int16) int16 {
	// Check for highest trump (spade)
	highTrumpRank := -1
	for i := 0; i < NumPlayers; i++ {
		if Suit(cardsPlayed[i]) == SuitSpades {
			if r := Rank(cardsPlayed[i]); r > highTrumpRank {
				highTrumpRank = r
			}
		}
	}

	var winCard byte
	if highTrumpRank >= 0 {
		winCard = byte(SuitSpades*13 + highTrumpRank)
	} else {
		// No trump — highest of led suit
		ledSuit := Suit(cardsPlayed[leadPlayer])
		highRank := -1
		for i := 0; i < NumPlayers; i++ {
			if Suit(cardsPlayed[i]) == ledSuit {
				if r := Rank(cardsPlayed[i]); r > highRank {
					highRank = r
				}
			}
		}
		winCard = byte(ledSuit*13 + highRank)
	}

	for i := 0; i < NumPlayers; i++ {
		if cardsPlayed[i] == winCard {
			return int16(i)
		}
	}
	return leadPlayer
}

func ValidBid(bid byte) bool {
	return bid <= 13 || bid == BidDoubleNil
}

// EffectiveBid returns the number of tricks a bid contributes to team total.
// Nil and double nil contribute 0 to the team's combined bid.
func EffectiveBid(bid byte) int {
	if bid == 0 || bid == BidDoubleNil {
		return 0
	}
	return int(bid)
}

// ScoreHand calculates per-team scoring for a completed hand.
func ScoreHand(
	bids [NumPlayers]byte,
	tricksWon [NumPlayers]int,
	bags [NumTeams]int,
) (score proto.SpadesHandScore, newBags [NumTeams]int) {
	newBags = bags

	for team := 0; team < NumTeams; team++ {
		seat0 := int16(team)
		seat1 := int16(team + 2)

		teamBid := EffectiveBid(bids[seat0]) + EffectiveBid(bids[seat1])
		teamTricks := tricksWon[seat0] + tricksWon[seat1]

		// Base score
		if teamBid == 0 {
			// Both players bid nil — no base score from tricks
			score.Base[team] = 0
		} else if teamTricks >= teamBid {
			score.Base[team] = int16(teamBid * 10)
			score.BagBonus[team] = int16(teamTricks - teamBid)
			newBags[team] += int(score.BagBonus[team])
		} else {
			// Set — failed to make bid
			score.Base[team] = int16(-teamBid * 10)
		}

		// Bag penalty
		if newBags[team] >= 10 {
			score.BagPenalty[team] = -100
			newBags[team] -= 10
		}

		// Nil bids
		for _, seat := range [2]int16{seat0, seat1} {
			if bids[seat] == 0 {
				if tricksWon[seat] == 0 {
					score.Nil[team] += 100
				} else {
					score.Nil[team] -= 100
				}
			} else if bids[seat] == BidDoubleNil {
				if tricksWon[seat] == 0 {
					score.Nil[team] += 200
				} else {
					score.Nil[team] -= 200
				}
			}
		}

		score.Scores[team] = score.Base[team] + score.BagBonus[team] + score.Nil[team] + score.BagPenalty[team]
	}

	return
}

func ValidCardToPlay(card byte, h *Hand, leadCard byte, isLeading bool, trumpsBroken bool) bool {
	if isLeading {
		if !trumpsBroken && Suit(card) == SuitSpades {
			// Can only lead spades if hand is all spades
			for i := 0; i < h.Count; i++ {
				if h.Cards[i] != CardNone && Suit(h.Cards[i]) != SuitSpades {
					return false
				}
			}
		}
		return true
	}

	// Following: must follow suit if possible
	ledSuit := Suit(leadCard)
	if Suit(card) != ledSuit {
		for i := 0; i < h.Count; i++ {
			if h.Cards[i] != CardNone && Suit(h.Cards[i]) == ledSuit {
				return false
			}
		}
	}
	return true
}

type Hand struct {
	Cards [proto.SpadesNumCardsInHand]byte
	Count int
}

func NewHand(cards [proto.SpadesNumCardsInHand]byte) Hand {
	h := Hand{Cards: cards}
	for _, c := range cards {
		if c != CardNone {
			h.Count++
		}
	}
	return h
}

func (h *Hand) Contains(card byte) bool {
	for i := 0; i < h.Count; i++ {
		if h.Cards[i] == card {
			return true
		}
	}
	return false
}

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
