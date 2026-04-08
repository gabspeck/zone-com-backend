package room

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"

	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// Table represents a game table with N seats.
type Table struct {
	mu         sync.Mutex
	ID         int16
	Seats      []*Player
	GameID     uint32
	Definition *GameDefinition
	Session    GameSession
	Status     int16 // 0=idle, 1=gaming
}

// SitDown seats a player at the table.
func (t *Table) SitDown(p *Player, seat int16) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sitDownLocked(p, seat)
}

// sitDownLocked is the lock-free core of SitDown. Caller must hold t.mu.
func (t *Table) sitDownLocked(p *Player, seat int16) bool {
	if int(seat) >= len(t.Seats) {
		log.Printf("[table] table %d: SitDown REJECTED for player %d at seat %d (out of range, %d seats)",
			t.ID, p.UserID, seat, len(t.Seats))
		return false
	}
	if t.Definition != nil && p.GameDef != t.Definition {
		log.Printf("[table] table %d: SitDown REJECTED for player %d due to game mismatch (%v != %v)",
			t.ID, p.UserID, gameKindName(p.GameDef), gameKindName(t.Definition))
		return false
	}
	if t.Seats[seat] != nil {
		log.Printf("[table] table %d: SitDown REJECTED for player %d at seat %d (occupied by %d)",
			t.ID, p.UserID, seat, t.Seats[seat].UserID)
		return false
	}
	if t.Definition == nil {
		t.Definition = p.GameDef
	}
	t.Seats[seat] = p
	p.Table = t
	p.Seat = seat
	log.Printf("[table] table %d: player %d (%s) sat down at seat %d", t.ID, p.UserID, p.UserName, seat)
	log.Printf("[table] table %d: seats now: %s", t.ID, t.seatsSummary())
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
	empty := true
	for _, s := range t.Seats {
		if s != nil {
			empty = false
			break
		}
	}
	if empty {
		log.Printf("[table] table %d: now empty, resetting to idle (was gameID=%d)", t.ID, t.GameID)
		t.Status = 0
		t.Session = nil
		t.GameID = 0
		t.Definition = nil
	}
	log.Printf("[table] table %d: seats now: %s", t.ID, t.seatsSummary())
}

// Opponent returns the other player at the table (for 2-player games).
func (t *Table) Opponent(seat int16) *Player {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.Seats) == 2 {
		opp := t.Seats[(seat+1)&1]
		if opp != nil {
			log.Printf("[table] table %d: opponent of seat %d is player %d (seat %d)", t.ID, seat, opp.UserID, (seat+1)&1)
		} else {
			log.Printf("[table] table %d: no opponent for seat %d", t.ID, seat)
		}
		return opp
	}
	// N-player: return first other player found
	for i, p := range t.Seats {
		if int16(i) != seat && p != nil {
			return p
		}
	}
	return nil
}

// AllSeated returns true if all seats are filled.
func (t *Table) AllSeated() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.Seats {
		if s == nil {
			log.Printf("[table] table %d: AllSeated=false %s", t.ID, t.seatsSummary())
			return false
		}
	}
	log.Printf("[table] table %d: AllSeated=true %s", t.ID, t.seatsSummary())
	return true
}

// StartGame initializes a new game on this table.
func (t *Table) StartGame(gameID uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.GameID = gameID
	if t.Definition != nil {
		t.Session = t.Definition.NewSession()
	}
	t.Status = 1
	log.Printf("[table] table %d: game %d initialized (%s)", t.ID, gameID, gameKindName(t.Definition))
}

// BroadcastGameMsg sends a game message wrapped in RoomGameMessage to all players at the table.
func (t *Table) BroadcastGameMsg(msgType uint32, payload []byte, excludeSeat int16) {
	t.mu.Lock()
	seats := make([]*Player, len(t.Seats))
	copy(seats, t.Seats)
	gameID := t.GameID
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
		msg := wire.AppMessage{
			Signature: proto.LobbySig,
			Channel:   p.Channel,
			Type:      proto.RoomMsgGameMessage,
			Data:      wrapped,
		}
		log.Printf("[broadcast] table %d: sending to seat %d (player %d, ch=%d)", t.ID, i, p.UserID, p.Channel)
		if err := p.Conn.WriteAppMessages([]wire.AppMessage{msg}); err != nil {
			log.Printf("[broadcast] table %d: send to seat %d (player %d) FAILED: %v", t.ID, i, p.UserID, err)
		}
	}
}

func gameKindName(def *GameDefinition) string {
	if def == nil {
		return "<none>"
	}
	return string(def.Kind)
}

// SendToSeat sends a game message to a specific seat.
func (t *Table) SendToSeat(seat int16, msgType uint32, payload []byte) {
	t.mu.Lock()
	if int(seat) >= len(t.Seats) {
		t.mu.Unlock()
		log.Printf("[sendToSeat] table %d: seat %d out of range (%d seats)", t.ID, seat, len(t.Seats))
		return
	}
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

func (t *Table) seatsSummary() string {
	parts := make([]string, len(t.Seats))
	for i, p := range t.Seats {
		parts[i] = seatInfo(p)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
