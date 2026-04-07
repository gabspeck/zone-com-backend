package room

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"zone.com/internal/checkers"
	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// Table represents a game table with 2 seats.
type Table struct {
	mu     sync.Mutex
	ID     int16
	Seats  [2]*Player
	GameID uint32
	Game   *checkers.Game
	Status int16 // 0=idle, 1=gaming
}

// SitDown seats a player at the table.
func (t *Table) SitDown(p *Player, seat int16) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Seats[seat] != nil {
		log.Printf("[table] table %d: SitDown REJECTED for player %d at seat %d (occupied by %d)",
			t.ID, p.UserID, seat, t.Seats[seat].UserID)
		return false
	}
	t.Seats[seat] = p
	p.Table = t
	p.Seat = seat
	log.Printf("[table] table %d: player %d (%s) sat down at seat %d", t.ID, p.UserID, p.UserName, seat)
	log.Printf("[table] table %d: seats now: [%s, %s]", t.ID, seatInfo(t.Seats[0]), seatInfo(t.Seats[1]))
	return true
}

func seatInfo(p *Player) string {
	if p == nil {
		return "empty"
	}
	return fmt.Sprintf("%d(%s)", p.UserID, p.UserName)
}

// RemovePlayer removes a player from the table.
func (t *Table) RemovePlayer(p *Player) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.Seats {
		if t.Seats[i] == p {
			log.Printf("[table] table %d: removing player %d from seat %d", t.ID, p.UserID, i)
			t.Seats[i] = nil
		}
	}
	if t.Seats[0] == nil && t.Seats[1] == nil {
		log.Printf("[table] table %d: now empty, resetting to idle (was gameID=%d)", t.ID, t.GameID)
		t.Status = 0
		t.Game = nil
		t.GameID = 0
	}
	log.Printf("[table] table %d: seats now: [%s, %s]", t.ID, seatInfo(t.Seats[0]), seatInfo(t.Seats[1]))
}

// Opponent returns the other player at the table, if any.
func (t *Table) Opponent(seat int16) *Player {
	t.mu.Lock()
	defer t.mu.Unlock()
	opp := t.Seats[(seat+1)&1]
	if opp != nil {
		log.Printf("[table] table %d: opponent of seat %d is player %d (seat %d)", t.ID, seat, opp.UserID, (seat+1)&1)
	} else {
		log.Printf("[table] table %d: no opponent for seat %d", t.ID, seat)
	}
	return opp
}

// BothSeated returns true if both seats are filled.
func (t *Table) BothSeated() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	both := t.Seats[0] != nil && t.Seats[1] != nil
	log.Printf("[table] table %d: BothSeated=%v [%s, %s]", t.ID, both, seatInfo(t.Seats[0]), seatInfo(t.Seats[1]))
	return both
}

// StartGame initializes a new game on this table.
func (t *Table) StartGame(gameID uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.GameID = gameID
	t.Game = checkers.NewGame()
	t.Game.State = checkers.StateWaitingForPlayers
	t.Status = 1
	log.Printf("[table] table %d: game %d initialized (state=WaitingForPlayers)", t.ID, gameID)
}

// BroadcastGameMsg sends a game message wrapped in RoomGameMessage to all players at the table.
func (t *Table) BroadcastGameMsg(msgType uint32, payload []byte, excludeSeat int16) {
	t.mu.Lock()
	seats := [2]*Player{t.Seats[0], t.Seats[1]}
	gameID := t.GameID
	channel := uint32(0)
	t.mu.Unlock()

	wrapped := proto.MarshalRoomGameMessage(gameID, msgType, payload)

	log.Printf("[broadcast] table %d: broadcasting gameMsg type=0x%x gameID=%d excludeSeat=%d payload=%s wrapped=%s",
		t.ID, msgType, gameID, excludeSeat, hex.EncodeToString(payload), hex.EncodeToString(wrapped))

	for i, p := range seats {
		if p == nil {
			log.Printf("[broadcast] table %d: seat %d is empty, skipping", t.ID, i)
			continue
		}
		if int16(i) == excludeSeat {
			log.Printf("[broadcast] table %d: seat %d (player %d) excluded", t.ID, i, p.UserID)
			continue
		}
		channel = p.Channel
		msg := wire.AppMessage{
			Signature: proto.LobbySig,
			Channel:   channel,
			Type:      proto.RoomMsgGameMessage,
			Data:      wrapped,
		}
		log.Printf("[broadcast] table %d: sending to seat %d (player %d, ch=%d)", t.ID, i, p.UserID, channel)
		if err := p.Conn.WriteAppMessages([]wire.AppMessage{msg}); err != nil {
			log.Printf("[broadcast] table %d: send to seat %d (player %d) FAILED: %v", t.ID, i, p.UserID, err)
		}
	}
}

// SendToSeat sends a game message to a specific seat.
func (t *Table) SendToSeat(seat int16, msgType uint32, payload []byte) {
	t.mu.Lock()
	p := t.Seats[seat]
	gameID := t.GameID
	t.mu.Unlock()

	if p == nil {
		log.Printf("[sendToSeat] table %d: seat %d is empty, not sending", t.ID, seat)
		return
	}

	wrapped := proto.MarshalRoomGameMessage(gameID, msgType, payload)
	msg := wire.AppMessage{
		Signature: proto.LobbySig,
		Channel:   p.Channel,
		Type:      proto.RoomMsgGameMessage,
		Data:      wrapped,
	}
	log.Printf("[sendToSeat] table %d: sending to seat %d (player %d): type=0x%x gameID=%d data=%s",
		t.ID, seat, p.UserID, msgType, gameID, hex.EncodeToString(wrapped))
	if err := p.Conn.WriteAppMessages([]wire.AppMessage{msg}); err != nil {
		log.Printf("[sendToSeat] table %d: send to seat %d FAILED: %v", t.ID, seat, err)
	}
}
