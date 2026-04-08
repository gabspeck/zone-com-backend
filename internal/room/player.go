package room

import (
	"zone.com/internal/conn"
)

// Player represents a connected player in the room.
type Player struct {
	UserID   uint32
	UserName string
	Service  string
	GameDef  *GameDefinition
	Conn     *conn.Conn
	Table    *Table
	Seat     int16
	Channel  uint32
	LCID     uint32
	Chat     bool
	Skill    int16
}
