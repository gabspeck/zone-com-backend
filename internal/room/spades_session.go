package room

import (
	"encoding/hex"
	"log"
	"sync"

	"zone.com/internal/proto"
	"zone.com/internal/spades"
)

type spadesSession struct {
	mu sync.Mutex

	// Player tracking
	ready    [spades.NumPlayers]bool
	readyCnt int

	// Game state
	hands      [spades.NumPlayers]spades.Hand
	teamScores [spades.NumTeams]int // cumulative
	bags       [spades.NumTeams]int
	handNum    int
	state      int16

	// Bidding phase
	bids   [spades.NumPlayers]byte
	bidCnt int

	// Play phase
	dealer       int16
	leadPlayer   int16
	playerToPlay int16
	cardsPlayed  [spades.NumPlayers]byte
	cardsPlayCnt int
	trickNum     int
	tricksWon    [spades.NumPlayers]int
	trumpsBroken bool

	// Rematch
	newGameVotes [spades.NumPlayers]bool
	newGameCnt   int
}

func newSpadesSession() GameSession {
	s := &spadesSession{
		state: proto.SpadesStateNone,
	}
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = spades.CardNone
	}
	for i := range s.bids {
		s.bids[i] = spades.BidNone
	}
	return s
}

func (s *spadesSession) MessageName(t uint32) string {
	switch t {
	case proto.SpadesMsgClientReady:
		return "ClientReady"
	case proto.SpadesMsgStartGame:
		return "StartGame"
	case proto.SpadesMsgReplacePlayer:
		return "ReplacePlayer"
	case proto.SpadesMsgStartBid:
		return "StartBid"
	case proto.SpadesMsgStartPlay:
		return "StartPlay"
	case proto.SpadesMsgEndHand:
		return "EndHand"
	case proto.SpadesMsgEndGame:
		return "EndGame"
	case proto.SpadesMsgBid:
		return "Bid"
	case proto.SpadesMsgPlay:
		return "Play"
	case proto.SpadesMsgNewGame:
		return "NewGame"
	case proto.SpadesMsgTalk:
		return "Talk"
	case proto.SpadesMsgOptions:
		return "Options"
	case proto.SpadesMsgShownCards:
		return "ShownCards"
	case proto.SpadesMsgGameStateReq:
		return "GameStateRequest"
	default:
		return "Unknown"
	}
}

func (s *spadesSession) HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[spades] player %d (seat %d): msg 0x%x (%s) len=%d data=%s",
		p.UserID, p.Seat, msgType, s.MessageName(msgType), len(payload), hex.EncodeToString(payload))

	switch msgType {
	case proto.SpadesMsgClientReady:
		return s.handleClientReady(table, p, payload)
	case proto.SpadesMsgBid:
		return s.handleBid(table, p, payload)
	case proto.SpadesMsgPlay:
		return s.handlePlay(table, p, payload)
	case proto.SpadesMsgNewGame:
		return s.handleNewGame(table, p, payload)
	case proto.SpadesMsgTalk:
		return s.handleTalk(table, p, payload)
	case proto.SpadesMsgCheckIn:
		log.Printf("[spades] player %d: CheckIn (ignored)", p.UserID)
	case proto.SpadesMsgShownCards:
		log.Printf("[spades] player %d: ShownCards (ignored)", p.UserID)
	case proto.SpadesMsgOptions:
		log.Printf("[spades] player %d: Options (ignored)", p.UserID)
	case proto.SpadesMsgGameStateReq:
		log.Printf("[spades] player %d: GameStateRequest (not implemented)", p.UserID)
	default:
		log.Printf("[spades] player %d: unhandled msg type 0x%x", p.UserID, msgType)
	}
	return nil
}

func (s *spadesSession) handleClientReady(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.SpadesClientReadySize {
		return nil
	}
	var msg proto.SpadesClientReady
	msg.Unmarshal(payload)
	log.Printf("[spades] player %d: ClientReady sig=0x%x ver=%d seat=%d",
		p.UserID, msg.ProtocolSignature, msg.ProtocolVersion, msg.Seat)

	if s.ready[p.Seat] {
		return nil
	}
	s.ready[p.Seat] = true
	s.readyCnt++

	if s.readyCnt == spades.NumPlayers {
		log.Printf("[spades] all %d players ready, sending StartGame", spades.NumPlayers)
		s.sendStartGame(table)
		s.startNewHand(table)
	}
	return nil
}

func (s *spadesSession) sendStartGame(table *Table) {
	msg := proto.SpadesStartGame{
		GameOptions:     0,
		NumPointsInGame: spades.PointsForGame,
		MinPointsInGame: spades.MinPoints,
	}
	for i := 0; i < spades.NumPlayers; i++ {
		if table.Seats[i] != nil {
			msg.Players[i] = table.Seats[i].UserID
		}
	}
	table.BroadcastGameMsg(proto.SpadesMsgStartGame, msg.Marshal(), -1)
}

func (s *spadesSession) startNewHand(table *Table) {
	s.dealer = int16(s.handNum % spades.NumPlayers)
	s.state = proto.SpadesStateBidding

	deck := spades.NewDeck()
	dealt := deck.Deal()

	for i := 0; i < spades.NumPlayers; i++ {
		s.hands[i] = spades.NewHand(dealt[i])
		s.bids[i] = spades.BidNone
		s.tricksWon[i] = 0
	}
	s.bidCnt = 0
	s.trickNum = 0
	s.trumpsBroken = false
	s.cardsPlayCnt = 0
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = spades.CardNone
	}
	s.playerToPlay = s.dealer

	// Per-player message: each player gets their own cards
	for i := int16(0); i < spades.NumPlayers; i++ {
		msg := proto.SpadesStartBid{
			BoardNumber: int16(s.handNum),
			Dealer:      s.dealer,
			Hand:        s.hands[i].Cards,
		}
		table.SendToSeat(i, proto.SpadesMsgStartBid, msg.Marshal())
	}
	log.Printf("[spades] hand %d started, dealer=seat %d", s.handNum, s.dealer)
}

func (s *spadesSession) handleBid(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.SpadesBidSize {
		return nil
	}
	if s.state != proto.SpadesStateBidding {
		log.Printf("[spades] player %d: Bid in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.SpadesBid
	msg.Unmarshal(payload)
	seat := p.Seat

	if seat != s.playerToPlay {
		log.Printf("[spades] player %d: not their turn to bid (expected seat %d)", p.UserID, s.playerToPlay)
		return nil
	}
	if !spades.ValidBid(msg.Bid) {
		log.Printf("[spades] player %d: invalid bid %d", p.UserID, msg.Bid)
		return nil
	}

	s.bids[seat] = msg.Bid
	s.bidCnt++
	nextBidder := (seat + 1) % spades.NumPlayers

	log.Printf("[spades] player %d (seat %d): bid %d (%d/%d)",
		p.UserID, seat, msg.Bid, s.bidCnt, spades.NumPlayers)

	// Broadcast bid to all
	outMsg := proto.SpadesBid{Seat: seat, NextBidder: nextBidder, Bid: msg.Bid}
	table.BroadcastGameMsg(proto.SpadesMsgBid, outMsg.Marshal(), -1)

	if s.bidCnt == spades.NumPlayers {
		s.beginPlay(table)
	} else {
		s.playerToPlay = nextBidder
	}
	return nil
}

func (s *spadesSession) beginPlay(table *Table) {
	s.state = proto.SpadesStatePlaying
	s.leadPlayer = s.dealer
	s.playerToPlay = s.dealer
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = spades.CardNone
	}
	s.cardsPlayCnt = 0

	msg := proto.SpadesStartPlay{Leader: s.dealer}
	table.BroadcastGameMsg(proto.SpadesMsgStartPlay, msg.Marshal(), -1)
	log.Printf("[spades] play started, leader=seat %d", s.dealer)
}

func (s *spadesSession) handlePlay(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.SpadesPlaySize {
		return nil
	}
	if s.state != proto.SpadesStatePlaying {
		log.Printf("[spades] player %d: Play in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.SpadesPlay
	msg.Unmarshal(payload)
	seat := p.Seat

	if seat != s.playerToPlay {
		log.Printf("[spades] player %d: not their turn (expected seat %d)", p.UserID, s.playerToPlay)
		return nil
	}
	if !s.hands[seat].Contains(msg.Card) {
		log.Printf("[spades] player %d: card %d not in hand", p.UserID, msg.Card)
		return nil
	}

	isLeading := s.cardsPlayCnt == 0
	var leadCard byte
	if !isLeading {
		leadCard = s.cardsPlayed[s.leadPlayer]
	}
	if !spades.ValidCardToPlay(msg.Card, &s.hands[seat], leadCard, isLeading, s.trumpsBroken) {
		log.Printf("[spades] player %d: invalid card %d to play", p.UserID, msg.Card)
		return nil
	}

	s.hands[seat].Remove(msg.Card)
	s.cardsPlayed[seat] = msg.Card
	s.cardsPlayCnt++

	if spades.Suit(msg.Card) == spades.SuitSpades {
		s.trumpsBroken = true
	}

	nextPlayer := (seat + 1) % spades.NumPlayers

	log.Printf("[spades] player %d (seat %d): played card %d (trick %d, %d/%d)",
		p.UserID, seat, msg.Card, s.trickNum, s.cardsPlayCnt, spades.NumPlayers)

	outMsg := proto.SpadesPlay{Seat: seat, NextPlayer: nextPlayer, Card: msg.Card}
	table.BroadcastGameMsg(proto.SpadesMsgPlay, outMsg.Marshal(), -1)

	if s.cardsPlayCnt == spades.NumPlayers {
		s.resolveTrick(table)
	} else {
		s.playerToPlay = nextPlayer
	}
	return nil
}

func (s *spadesSession) resolveTrick(table *Table) {
	winner := spades.TrickWinner(s.cardsPlayed, s.leadPlayer)
	s.tricksWon[winner]++
	log.Printf("[spades] trick %d won by seat %d", s.trickNum, winner)

	s.trickNum++
	s.leadPlayer = winner
	s.playerToPlay = winner
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = spades.CardNone
	}
	s.cardsPlayCnt = 0

	if s.trickNum == spades.CardsPerPlayer {
		s.endHand(table)
	}
}

func (s *spadesSession) endHand(table *Table) {
	score, newBags := spades.ScoreHand(s.bids, s.tricksWon, s.bags)
	score.BoardNumber = int16(s.handNum)

	for i := 0; i < spades.NumTeams; i++ {
		s.teamScores[i] += int(score.Scores[i])
		s.bags[i] = newBags[i]
	}

	msg := proto.SpadesEndHand{
		Bags:  [proto.SpadesNumTeams]int16{int16(newBags[0]), int16(newBags[1])},
		Score: score,
	}
	table.BroadcastGameMsg(proto.SpadesMsgEndHand, msg.Marshal(), -1)

	log.Printf("[spades] hand %d ended: team scores=%v bags=%v cumulative=%v",
		s.handNum, score.Scores, newBags, s.teamScores)

	s.handNum++

	gameOver := false
	for i := 0; i < spades.NumTeams; i++ {
		if s.teamScores[i] >= spades.PointsForGame || s.teamScores[i] <= spades.MinPoints {
			gameOver = true
			break
		}
	}

	if gameOver {
		s.endGame(table)
	} else {
		s.startNewHand(table)
	}
}

func (s *spadesSession) endGame(table *Table) {
	s.state = proto.SpadesStateEndGame

	var msg proto.SpadesEndGame
	if s.teamScores[0] > s.teamScores[1] {
		msg.Winners = [proto.SpadesNumPlayers]byte{1, 0, 1, 0} // team 0 wins
	} else {
		msg.Winners = [proto.SpadesNumPlayers]byte{0, 1, 0, 1} // team 1 wins
	}
	table.BroadcastGameMsg(proto.SpadesMsgEndGame, msg.Marshal(), -1)
	log.Printf("[spades] game over! team scores=%v winners=%v", s.teamScores, msg.Winners)

	for i := range s.newGameVotes {
		s.newGameVotes[i] = false
	}
	s.newGameCnt = 0
}

func (s *spadesSession) handleNewGame(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.SpadesNewGameSize {
		return nil
	}
	if s.state != proto.SpadesStateEndGame {
		log.Printf("[spades] player %d: NewGame in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.SpadesNewGame
	msg.Unmarshal(payload)
	seat := p.Seat

	if s.newGameVotes[seat] {
		return nil
	}
	s.newGameVotes[seat] = true
	s.newGameCnt++
	log.Printf("[spades] player %d: voted for new game (%d/%d)", p.UserID, s.newGameCnt, spades.NumPlayers)

	if s.newGameCnt == spades.NumPlayers {
		log.Printf("[spades] all players voted, starting new game")
		for i := range s.teamScores {
			s.teamScores[i] = 0
		}
		for i := range s.bags {
			s.bags[i] = 0
		}
		s.handNum = 0
		s.state = proto.SpadesStateNone
		s.sendStartGame(table)
		s.startNewHand(table)
	}
	return nil
}

func (s *spadesSession) handleTalk(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.SpadesTalkHdrSize {
		return nil
	}
	table.BroadcastGameMsg(proto.SpadesMsgTalk, payload, -1)
	return nil
}
