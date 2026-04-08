package room

import (
	"encoding/hex"
	"log"
	"sync"

	"zone.com/internal/hearts"
	"zone.com/internal/proto"
)

type heartsSession struct {
	mu sync.Mutex

	// Player tracking
	ready     [hearts.NumPlayers]bool
	readyCnt  int
	checkedIn [hearts.NumPlayers]bool

	// Game state
	hands   [hearts.NumPlayers]hearts.Hand
	scores  [hearts.NumPlayers]int // cumulative
	handNum int
	passDir int16
	state   int16

	// Pass phase
	passCards [hearts.NumPlayers][proto.HeartsMaxNumCardsInPass]byte
	passed    [hearts.NumPlayers]bool
	passCnt   int

	// Play phase
	leadPlayer   int16
	playerToPlay int16
	cardsPlayed  [hearts.NumPlayers]byte
	cardsPlayCnt int
	trickNum     int
	pointsBroken bool
	trickCards   [hearts.NumPlayers][]byte // cards won per seat this hand

	// Rematch
	newGameVotes [hearts.NumPlayers]bool
	newGameCnt   int
}

func newHeartsSession() GameSession {
	s := &heartsSession{
		state: proto.HeartsStateNone,
	}
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = hearts.CardNone
	}
	return s
}

func (s *heartsSession) MessageName(t uint32) string {
	switch t {
	case proto.HeartsMsgStartGame:
		return "StartGame"
	case proto.HeartsMsgReplacePlayer:
		return "ReplacePlayer"
	case proto.HeartsMsgStartHand:
		return "StartHand"
	case proto.HeartsMsgStartPlay:
		return "StartPlay"
	case proto.HeartsMsgEndHand:
		return "EndHand"
	case proto.HeartsMsgEndGame:
		return "EndGame"
	case proto.HeartsMsgClientReady:
		return "ClientReady"
	case proto.HeartsMsgPassCards:
		return "PassCards"
	case proto.HeartsMsgPlayCard:
		return "PlayCard"
	case proto.HeartsMsgNewGame:
		return "NewGame"
	case proto.HeartsMsgTalk:
		return "Talk"
	case proto.HeartsMsgCheckIn:
		return "CheckIn"
	case proto.HeartsMsgOptions:
		return "Options"
	case proto.HeartsMsgGameStateReq:
		return "GameStateRequest"
	default:
		return "Unknown"
	}
}

func (s *heartsSession) HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[hearts] player %d (seat %d): msg 0x%x (%s) len=%d data=%s",
		p.UserID, p.Seat, msgType, s.MessageName(msgType), len(payload), hex.EncodeToString(payload))

	switch msgType {
	case proto.HeartsMsgClientReady:
		return s.handleClientReady(table, p, payload)
	case proto.HeartsMsgCheckIn:
		return s.handleCheckIn(table, p, payload)
	case proto.HeartsMsgPassCards:
		return s.handlePassCards(table, p, payload)
	case proto.HeartsMsgPlayCard:
		return s.handlePlayCard(table, p, payload)
	case proto.HeartsMsgNewGame:
		return s.handleNewGame(table, p, payload)
	case proto.HeartsMsgTalk:
		return s.handleTalk(table, p, payload)
	case proto.HeartsMsgOptions:
		log.Printf("[hearts] player %d: Options received (ignored)", p.UserID)
	case proto.HeartsMsgGameStateReq:
		log.Printf("[hearts] player %d: GameStateRequest (not implemented)", p.UserID)
	default:
		log.Printf("[hearts] player %d: unhandled msg type 0x%x", p.UserID, msgType)
	}
	return nil
}

func (s *heartsSession) handleCheckIn(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsCheckInSize {
		return nil
	}
	var msg proto.HeartsCheckIn
	msg.Unmarshal(payload)
	log.Printf("[hearts] player %d: CheckIn userID=%d seat=%d", p.UserID, msg.UserID, msg.Seat)
	s.checkedIn[p.Seat] = true
	return nil
}

func (s *heartsSession) handleClientReady(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsClientReadySize {
		return nil
	}
	var msg proto.HeartsClientReady
	msg.Unmarshal(payload)
	log.Printf("[hearts] player %d: ClientReady sig=0x%x ver=%d seat=%d",
		p.UserID, msg.ProtocolSignature, msg.ProtocolVersion, msg.Seat)

	if s.ready[p.Seat] {
		log.Printf("[hearts] player %d: already ready, ignoring", p.UserID)
		return nil
	}
	s.ready[p.Seat] = true
	s.readyCnt++

	if s.readyCnt == hearts.NumPlayers {
		log.Printf("[hearts] all %d players ready, sending StartGame", hearts.NumPlayers)
		s.sendStartGame(table)
		s.startNewHand(table)
	}
	return nil
}

func (s *heartsSession) sendStartGame(table *Table) {
	msg := proto.HeartsStartGame{
		NumCardsInHand:  hearts.CardsPerPlayer,
		NumCardsInPass:  hearts.CardsToPass,
		NumPointsInGame: hearts.PointsForGame,
		GameOptions:     0,
	}
	for i := 0; i < hearts.NumPlayers; i++ {
		if table.Seats[i] != nil {
			msg.Players[i] = table.Seats[i].UserID
		}
	}
	table.BroadcastGameMsg(proto.HeartsMsgStartGame, msg.Marshal(), -1)
}

func (s *heartsSession) startNewHand(table *Table) {
	s.passDir = hearts.PassDirs[s.handNum%len(hearts.PassDirs)]

	deck := hearts.NewDeck()
	dealt := deck.Deal()

	for i := 0; i < hearts.NumPlayers; i++ {
		s.hands[i] = hearts.NewHand(dealt[i])
		s.passed[i] = false
		s.trickCards[i] = nil
	}
	s.passCnt = 0
	s.trickNum = 0
	s.pointsBroken = false
	s.cardsPlayCnt = 0
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = hearts.CardNone
	}

	if s.passDir == proto.HeartsPassHold {
		s.state = proto.HeartsStatePlayCards
	} else {
		s.state = proto.HeartsStatePassCards
	}

	// Per-player message: each player gets their own cards
	for i := int16(0); i < hearts.NumPlayers; i++ {
		msg := proto.HeartsStartHand{
			PassDir: s.passDir,
			Cards:   s.hands[i].Cards,
		}
		table.SendToSeat(i, proto.HeartsMsgStartHand, msg.Marshal())
	}
	log.Printf("[hearts] hand %d started, passDir=%d", s.handNum, s.passDir)

	if s.passDir == proto.HeartsPassHold {
		s.beginPlay(table)
	}
}

func (s *heartsSession) handlePassCards(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsPassCardsSize {
		return nil
	}
	if s.state != proto.HeartsStatePassCards {
		log.Printf("[hearts] player %d: PassCards in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.HeartsPassCards
	msg.Unmarshal(payload)
	seat := p.Seat

	if s.passed[seat] {
		log.Printf("[hearts] player %d: already passed, ignoring", p.UserID)
		return nil
	}

	for i := 0; i < hearts.CardsToPass; i++ {
		if !s.hands[seat].Contains(msg.Pass[i]) {
			log.Printf("[hearts] player %d: pass card %d not in hand", p.UserID, msg.Pass[i])
			return nil
		}
	}

	s.passCards[seat] = msg.Pass
	s.passed[seat] = true
	s.passCnt++
	log.Printf("[hearts] player %d: passed cards [%d, %d, %d] (%d/%d)",
		p.UserID, msg.Pass[0], msg.Pass[1], msg.Pass[2], s.passCnt, hearts.NumPlayers)

	if s.passCnt == hearts.NumPlayers {
		s.resolvePass(table)
	}
	return nil
}

func (s *heartsSession) resolvePass(table *Table) {
	for from := int16(0); from < hearts.NumPlayers; from++ {
		target := hearts.PassTarget(from, s.passDir)
		for i := 0; i < hearts.CardsToPass; i++ {
			card := s.passCards[from][i]
			s.hands[from].Remove(card)
			s.hands[target].Add(card)
		}
	}

	for from := int16(0); from < hearts.NumPlayers; from++ {
		msg := proto.HeartsPassCards{
			Seat: from,
			Pass: s.passCards[from],
		}
		table.BroadcastGameMsg(proto.HeartsMsgPassCards, msg.Marshal(), -1)
	}

	s.state = proto.HeartsStatePlayCards
	s.beginPlay(table)
}

func (s *heartsSession) beginPlay(table *Table) {
	// Find who holds 2C after pass phase
	s.leadPlayer = s.find2CHolder()
	s.playerToPlay = s.leadPlayer
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = hearts.CardNone
	}
	s.cardsPlayCnt = 0

	msg := proto.HeartsStartPlay{Seat: s.leadPlayer}
	table.BroadcastGameMsg(proto.HeartsMsgStartPlay, msg.Marshal(), -1)
	log.Printf("[hearts] play started, 2C holder = seat %d", s.leadPlayer)
}

func (s *heartsSession) find2CHolder() int16 {
	for seat := int16(0); seat < hearts.NumPlayers; seat++ {
		if s.hands[seat].Contains(hearts.Card2C) {
			return seat
		}
	}
	return 0
}

func (s *heartsSession) handlePlayCard(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsPlayCardSize {
		return nil
	}
	if s.state != proto.HeartsStatePlayCards {
		log.Printf("[hearts] player %d: PlayCard in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.HeartsPlayCard
	msg.Unmarshal(payload)
	seat := p.Seat

	if seat != s.playerToPlay {
		log.Printf("[hearts] player %d: not their turn (expected seat %d)", p.UserID, s.playerToPlay)
		return nil
	}
	if !s.hands[seat].Contains(msg.Card) {
		log.Printf("[hearts] player %d: card %d not in hand", p.UserID, msg.Card)
		return nil
	}

	s.hands[seat].Remove(msg.Card)
	s.cardsPlayed[seat] = msg.Card
	s.cardsPlayCnt++

	if hearts.IsPointCard(msg.Card) {
		s.pointsBroken = true
	}

	log.Printf("[hearts] player %d: played card %d (trick %d, %d/%d played)",
		p.UserID, msg.Card, s.trickNum, s.cardsPlayCnt, hearts.NumPlayers)

	outMsg := proto.HeartsPlayCard{Seat: seat, Card: msg.Card}
	table.BroadcastGameMsg(proto.HeartsMsgPlayCard, outMsg.Marshal(), -1)

	if s.cardsPlayCnt == hearts.NumPlayers {
		s.resolveTrick(table)
	} else {
		s.playerToPlay = (s.playerToPlay + 1) % hearts.NumPlayers
	}
	return nil
}

func (s *heartsSession) resolveTrick(table *Table) {
	winner := hearts.TrickWinner(s.cardsPlayed, s.leadPlayer)
	log.Printf("[hearts] trick %d won by seat %d", s.trickNum, winner)

	for i := 0; i < hearts.NumPlayers; i++ {
		s.trickCards[winner] = append(s.trickCards[winner], s.cardsPlayed[i])
	}

	s.trickNum++
	s.leadPlayer = winner
	s.playerToPlay = winner
	for i := range s.cardsPlayed {
		s.cardsPlayed[i] = hearts.CardNone
	}
	s.cardsPlayCnt = 0

	if s.trickNum == hearts.CardsPerPlayer {
		s.endHand(table)
	}
}

func (s *heartsSession) endHand(table *Table) {
	handScores, runPlayer := hearts.ScoreHand(s.trickCards)

	msg := proto.HeartsEndHand{RunPlayer: runPlayer}
	for i := 0; i < hearts.NumPlayers; i++ {
		s.scores[i] += handScores[i]
		msg.Score[i] = int16(handScores[i])
	}
	log.Printf("[hearts] hand %d ended: scores=%v run=%d cumulative=%v",
		s.handNum, handScores, runPlayer, s.scores)

	table.BroadcastGameMsg(proto.HeartsMsgEndHand, msg.Marshal(), -1)

	s.handNum++

	gameOver := false
	for i := 0; i < hearts.NumPlayers; i++ {
		if s.scores[i] >= hearts.PointsForGame {
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

func (s *heartsSession) endGame(table *Table) {
	s.state = proto.HeartsStateEndGame
	msg := proto.HeartsEndGame{Forfeiter: -1}
	table.BroadcastGameMsg(proto.HeartsMsgEndGame, msg.Marshal(), -1)
	log.Printf("[hearts] game over! final scores=%v", s.scores)

	for i := range s.newGameVotes {
		s.newGameVotes[i] = false
	}
	s.newGameCnt = 0
}

func (s *heartsSession) handleNewGame(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsNewGameSize {
		return nil
	}
	if s.state != proto.HeartsStateEndGame {
		log.Printf("[hearts] player %d: NewGame in wrong state %d", p.UserID, s.state)
		return nil
	}
	var msg proto.HeartsNewGame
	msg.Unmarshal(payload)
	seat := p.Seat

	if s.newGameVotes[seat] {
		return nil
	}
	s.newGameVotes[seat] = true
	s.newGameCnt++
	log.Printf("[hearts] player %d: voted for new game (%d/%d)", p.UserID, s.newGameCnt, hearts.NumPlayers)

	if s.newGameCnt == hearts.NumPlayers {
		log.Printf("[hearts] all players voted, starting new game")
		for i := range s.scores {
			s.scores[i] = 0
		}
		s.handNum = 0
		s.state = proto.HeartsStateNone
		s.sendStartGame(table)
		s.startNewHand(table)
	}
	return nil
}

func (s *heartsSession) handleTalk(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.HeartsTalkHdrSize {
		return nil
	}
	table.BroadcastGameMsg(proto.HeartsMsgTalk, payload, -1)
	return nil
}
