package room

import (
	"encoding/hex"
	"log"
	"sync"

	"zone.com/internal/conn"
	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// Room manages tables and players for the game server.
type Room struct {
	mu         sync.RWMutex
	tables     []*Table
	players    map[uint32]*Player
	nextUserID uint32
	nextGameID uint32
	numSeats   int
}

// New creates a new room with the given number of tables.
func New(numTables, maxSeatsPerTable int) *Room {
	r := &Room{
		tables:     make([]*Table, numTables),
		players:    make(map[uint32]*Player),
		nextUserID: 100, // start at 100 to avoid collisions with special IDs
		nextGameID: 1,
		numSeats:   maxSeatsPerTable,
	}
	for i := range r.tables {
		r.tables[i] = &Table{
			ID:    int16(i),
			Seats: make([]*Player, maxSeatsPerTable),
		}
	}
	return r
}

// NumTables returns the number of tables.
func (r *Room) NumTables() int {
	return len(r.tables)
}

// NumSeats returns seats per table.
func (r *Room) NumSeats() int {
	return r.numSeats
}

// AddPlayer registers a new player and assigns a userID.
func (r *Room) AddPlayer(c *conn.Conn, userName string, channel uint32, lcid uint32, chat bool, skill int16) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	uid := r.nextUserID
	r.nextUserID++

	p := &Player{
		UserID:   uid,
		UserName: userName,
		Conn:     c,
		Seat:     -1,
		Channel:  channel,
		LCID:     lcid,
		Chat:     chat,
		Skill:    skill,
	}
	r.players[uid] = p
	log.Printf("[room] AddPlayer: userID=%d name=%q channel=%d (total players: %d)", uid, userName, channel, len(r.players))
	return p
}

func (r *Room) BroadcastEnter(newPlayer *Player) {
	r.mu.RLock()
	players := make([]*Player, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, p)
	}
	r.mu.RUnlock()

	newPlayerEnter := proto.MarshalRoomEnter(newPlayer.UserID, newPlayer.UserName)
	for _, other := range players {
		if other.UserID == newPlayer.UserID {
			continue
		}

		otherEnter := proto.MarshalRoomEnter(other.UserID, other.UserName)
		log.Printf("[room] BroadcastEnter: sending existing player %d to new player %d data=%s",
			other.UserID, newPlayer.UserID, hex.EncodeToString(otherEnter))
		if err := newPlayer.Conn.WriteAppMessages([]wire.AppMessage{{
			Signature: proto.LobbySig,
			Channel:   newPlayer.Channel,
			Type:      proto.RoomMsgEnter,
			Data:      otherEnter,
		}}); err != nil {
			log.Printf("[room] BroadcastEnter: send existing player %d to new player %d FAILED: %v",
				other.UserID, newPlayer.UserID, err)
		}

		log.Printf("[room] BroadcastEnter: announcing new player %d to player %d data=%s",
			newPlayer.UserID, other.UserID, hex.EncodeToString(newPlayerEnter))
		if err := other.Conn.WriteAppMessages([]wire.AppMessage{{
			Signature: proto.LobbySig,
			Channel:   other.Channel,
			Type:      proto.RoomMsgEnter,
			Data:      newPlayerEnter,
		}}); err != nil {
			log.Printf("[room] BroadcastEnter: announce new player %d to player %d FAILED: %v",
				newPlayer.UserID, other.UserID, err)
		}
	}
}

func (r *Room) WaitingPlayers() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	waiting := 0
	for _, p := range r.players {
		if p.Table == nil {
			waiting++
			continue
		}
		p.Table.mu.Lock()
		allSeated := true
		for _, s := range p.Table.Seats {
			if s == nil {
				allSeated = false
				break
			}
		}
		p.Table.mu.Unlock()
		if !allSeated {
			waiting++
		}
	}
	return waiting
}

// RemovePlayer removes a player from the room and any table.
func (r *Room) RemovePlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("[room] RemovePlayer: userID=%d name=%q", p.UserID, p.UserName)
	if p.Table != nil {
		log.Printf("[room] RemovePlayer: player %d leaving table %d", p.UserID, p.Table.ID)
		p.Table.RemovePlayer(p)
	}
	delete(r.players, p.UserID)
	log.Printf("[room] RemovePlayer: done (total players: %d)", len(r.players))
}

// FindSeat finds a table that needs one more player for the same game,
// or seats them at an empty table. Returns the table and seat.
func (r *Room) FindSeat(p *Player) (*Table, int16) {
	r.mu.Lock()
	defer r.mu.Unlock()

	neededSeats := int(p.GameDef.Seats)
	log.Printf("[room] FindSeat: looking for seat for player %d (%s), game needs %d seats...",
		p.UserID, p.UserName, neededSeats)

	// First: find a table with the most players that still has room (matchmaking priority)
	bestTable := (*Table)(nil)
	bestCount := 0
	for _, t := range r.tables {
		t.mu.Lock()
		if t.Definition != nil && t.Definition != p.GameDef {
			t.mu.Unlock()
			continue
		}
		seated := 0
		for _, s := range t.Seats {
			if s != nil {
				seated++
			}
		}
		hasRoom := seated > 0 && seated < neededSeats && len(t.Seats) >= neededSeats
		t.mu.Unlock()
		if hasRoom && seated > bestCount {
			bestTable = t
			bestCount = seated
		}
	}
	if bestTable != nil {
		bestTable.mu.Lock()
		if len(bestTable.Seats) != neededSeats {
			old := bestTable.Seats
			bestTable.Seats = make([]*Player, neededSeats)
			copy(bestTable.Seats, old[:min(len(old), neededSeats)])
		}
		for seat := int16(0); seat < int16(neededSeats); seat++ {
			if bestTable.sitDownLocked(p, seat) {
				bestTable.mu.Unlock()
				log.Printf("[room] FindSeat: matched player %d to table %d seat %d (matchmaking)", p.UserID, bestTable.ID, seat)
				return bestTable, seat
			}
		}
		bestTable.mu.Unlock()
	}

	log.Printf("[room] FindSeat: no partially filled table, looking for empty table...")

	// Second: find a completely empty table
	for _, t := range r.tables {
		t.mu.Lock()
		if t.Definition != nil && t.Definition != p.GameDef {
			t.mu.Unlock()
			continue
		}
		if len(t.Seats) != neededSeats {
			t.Seats = make([]*Player, neededSeats)
		}
		if t.sitDownLocked(p, 0) {
			t.mu.Unlock()
			log.Printf("[room] FindSeat: seated player %d at empty table %d seat 0", p.UserID, t.ID)
			return t, 0
		}
		t.mu.Unlock()
	}

	log.Printf("[room] FindSeat: NO tables available for player %d!", p.UserID)
	return nil, -1
}

// NextGameID allocates a new game ID.
func (r *Room) NextGameID() uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	gid := r.nextGameID
	r.nextGameID++
	log.Printf("[room] NextGameID: allocated game ID %d", gid)
	return gid
}

func (r *Room) BroadcastChatSwitch(p *Player) {
	r.mu.RLock()
	players := make([]*Player, 0, len(r.players))
	for _, other := range r.players {
		players = append(players, other)
	}
	r.mu.RUnlock()

	data := proto.MarshalRoomChatSwitch(p.UserID, p.Chat)
	for _, other := range players {
		msg := wire.AppMessage{
			Signature: proto.LobbySig,
			Channel:   other.Channel,
			Type:      proto.RoomMsgChatSwitch,
			Data:      data,
		}
		log.Printf("[room] BroadcastChatSwitch: userID=%d chat=%v -> player %d channel=%d",
			p.UserID, p.Chat, other.UserID, other.Channel)
		if err := other.Conn.WriteAppMessages([]wire.AppMessage{msg}); err != nil {
			log.Printf("[room] BroadcastChatSwitch: send to player %d FAILED: %v", other.UserID, err)
		}
	}
}
