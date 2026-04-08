package proto

import "zone.com/internal/wire"

// Room message struct sizes
const (
	RoomUserInfoSize     = 72 // sig(4)+ver(4)+clientVer(4)+internalName(32)+userName(32)
	RoomClientConfigSize = 264
	RoomAccessedSize     = 24 // userID(4)+numTables(2)+numSeats(2)+gameOpts(4)+groupID(4)+privs(4) [protocol 17]
	RoomEnterSize        = 56 // userID(4)+userName(32)+hostAddr(4)+timeSuspended(4)+latency(4)+rating(2)+gamesPlayed(2)+gamesAbandoned(2)+rfu(2)
	RoomSeatRequestSize  = 12 // userID(4)+table(2)+seat(2)+action(2)+rfu(2)
	RoomSeatResponseSize = 16 // userID(4)+gameID(4)+table(2)+seat(2)+action(2)+rfu(2)
	RoomStartGameSize    = 8  // gameID(4)+table(2)+seat(2)
	RoomStartGameMSize   = 36
	RoomGameMessageHdr   = 12 // gameID(4)+messageType(4)+messageLen(2)+rfu(2)
	RoomPingSize         = 8  // userID(4)+pingTime(4)
	RoomLeaveSize        = 4  // userID(4)
	RoomZUserIDRespSize  = 40
	RoomServerStatusSize = 8
	RoomChatSwitchSize   = 8 // userID(4)+fChat(1)+pad(3)
)

// RoomUserInfo is the client check-in message.
type RoomUserInfo struct {
	ProtocolSignature uint32
	ProtocolVersion   uint32
	ClientVersion     uint32
	InternalName      [GameIDLen + 1]byte
	UserName          [UserNameLen + 1]byte
}

func (m *RoomUserInfo) Unmarshal(b []byte) {
	m.ProtocolSignature = wire.ReadLE32(b[0:])
	m.ProtocolVersion = wire.ReadLE32(b[4:])
	m.ClientVersion = wire.ReadLE32(b[8:])
	copy(m.InternalName[:], b[12:12+GameIDLen+1])
	copy(m.UserName[:], b[44:44+UserNameLen+1])
}

type RoomClientConfig struct {
	ProtocolSignature uint32
	ProtocolVersion   uint32
	Config            [256]byte
}

func (m *RoomClientConfig) Unmarshal(b []byte) {
	m.ProtocolSignature = wire.ReadLE32(b[0:])
	m.ProtocolVersion = wire.ReadLE32(b[4:])
	copy(m.Config[:], b[8:8+256])
}

// MarshalRoomAccessed builds the server Accessed response.
func MarshalRoomAccessed(userID uint32, numTables, numSeats uint16, gameOptions, groupID, privs uint32) []byte {
	b := make([]byte, RoomAccessedSize)
	wire.WriteLE32(b[0:], userID)
	wire.WriteLE16(b[4:], numTables)
	wire.WriteLE16(b[6:], numSeats)
	wire.WriteLE32(b[8:], gameOptions)
	wire.WriteLE32(b[12:], groupID)
	wire.WriteLE32(b[16:], privs) // protocol 17: maskRoomCmdPrivs
	// 4 bytes padding at 20 to reach 24 -- actually no, sizeof is 24 with the 6 fields
	return b
}

func MarshalRoomEnter(userID uint32, userName string) []byte {
	b := make([]byte, RoomEnterSize)
	wire.WriteLE32(b[0:], userID)
	copy(b[4:36], append([]byte(userName), 0))
	wire.WriteLE32(b[36:], 0) // hostAddr
	wire.WriteLE32(b[40:], 0) // timeSuspended
	wire.WriteLE32(b[44:], 0) // latency
	wire.WriteLE16(b[48:], uint16(0xffff))
	wire.WriteLE16(b[50:], uint16(0xffff))
	wire.WriteLE16(b[52:], uint16(0xffff))
	wire.WriteLE16(b[54:], 0)
	return b
}

// RoomSeatRequest is client->server seat action.
type RoomSeatRequest struct {
	UserID uint32
	Table  int16
	Seat   int16
	Action int16
	Rfu    int16
}

func (m *RoomSeatRequest) Unmarshal(b []byte) {
	m.UserID = wire.ReadLE32(b[0:])
	m.Table = int16(wire.ReadLE16(b[4:]))
	m.Seat = int16(wire.ReadLE16(b[6:]))
	m.Action = int16(wire.ReadLE16(b[8:]))
	m.Rfu = int16(wire.ReadLE16(b[10:]))
}

// MarshalRoomSeatResponse builds the server seat response.
func MarshalRoomSeatResponse(userID, gameID uint32, table, seat, action int16) []byte {
	b := make([]byte, RoomSeatResponseSize)
	wire.WriteLE32(b[0:], userID)
	wire.WriteLE32(b[4:], gameID)
	wire.WriteLE16(b[8:], uint16(table))
	wire.WriteLE16(b[10:], uint16(seat))
	wire.WriteLE16(b[12:], uint16(action))
	wire.WriteLE16(b[14:], 0) // rfu
	return b
}

// MarshalRoomStartGame builds the StartGame message.
func MarshalRoomStartGame(gameID uint32, table, seat int16) []byte {
	b := make([]byte, RoomStartGameSize)
	wire.WriteLE32(b[0:], gameID)
	wire.WriteLE16(b[4:], uint16(table))
	wire.WriteLE16(b[6:], uint16(seat))
	return b
}

type RoomStartGameMPlayer struct {
	UserID uint32
	LCID   uint32
	Chat   bool
	Skill  int16
}

func MarshalRoomStartGameM(gameID uint32, table, seat int16, players [2]RoomStartGameMPlayer) []byte {
	b := make([]byte, RoomStartGameMSize)
	wire.WriteLE32(b[0:], gameID)
	wire.WriteLE16(b[4:], uint16(table))
	wire.WriteLE16(b[6:], uint16(seat))
	wire.WriteLE16(b[8:], 2)
	off := 12
	for _, p := range players {
		wire.WriteLE32(b[off+0:], p.UserID)
		wire.WriteLE32(b[off+4:], p.LCID)
		if p.Chat {
			b[off+8] = 1
		}
		wire.WriteLE16(b[off+10:], uint16(p.Skill))
		off += 12
	}
	return b
}

func MarshalRoomZUserIDResponse(userID uint32, userName string, lcid uint32) []byte {
	b := make([]byte, RoomZUserIDRespSize)
	wire.WriteLE32(b[0:], userID)
	copy(b[4:36], append([]byte(userName), 0))
	wire.WriteLE32(b[36:], lcid)
	return b
}

func MarshalRoomServerStatus(status, playersWaiting uint32) []byte {
	b := make([]byte, RoomServerStatusSize)
	wire.WriteLE32(b[0:], status)
	wire.WriteLE32(b[4:], playersWaiting)
	return b
}

type RoomChatSwitch struct {
	UserID uint32
	Chat   bool
}

func (m *RoomChatSwitch) Unmarshal(b []byte) {
	m.UserID = wire.ReadLE32(b[0:])
	m.Chat = len(b) > 4 && b[4] != 0
}

func MarshalRoomChatSwitch(userID uint32, chat bool) []byte {
	b := make([]byte, RoomChatSwitchSize)
	wire.WriteLE32(b[0:], userID)
	if chat {
		b[4] = 1
	}
	return b
}

// RoomGameMessage is the wrapper for game-specific messages.
type RoomGameMessage struct {
	GameID      uint32
	MessageType uint32
	MessageLen  uint16
	Rfu         int16
}

func (m *RoomGameMessage) Unmarshal(b []byte) {
	m.GameID = wire.ReadLE32(b[0:])
	m.MessageType = wire.ReadLE32(b[4:])
	m.MessageLen = wire.ReadLE16(b[8:])
	m.Rfu = int16(wire.ReadLE16(b[10:]))
}

// MarshalRoomGameMessage builds the wrapper header + game payload.
func MarshalRoomGameMessage(gameID, messageType uint32, payload []byte) []byte {
	b := make([]byte, RoomGameMessageHdr+len(payload))
	wire.WriteLE32(b[0:], gameID)
	wire.WriteLE32(b[4:], messageType)
	wire.WriteLE16(b[8:], uint16(len(payload)))
	wire.WriteLE16(b[10:], 0) // rfu
	copy(b[RoomGameMessageHdr:], payload)
	return b
}

// MarshalRoomPing builds a ping response.
func MarshalRoomPing(userID, pingTime uint32) []byte {
	b := make([]byte, RoomPingSize)
	wire.WriteLE32(b[0:], userID)
	wire.WriteLE32(b[4:], pingTime)
	return b
}

// MarshalRoomLeave builds a leave notification.
func MarshalRoomLeave(userID uint32) []byte {
	b := make([]byte, RoomLeaveSize)
	wire.WriteLE32(b[0:], userID)
	return b
}

// MarshalRoomRoomInfo builds a minimal RoomInfo message.
// For the Whistler client, we send a simplified version with 0 players and 0 table infos.
func MarshalRoomRoomInfo(userID uint32, numTables, numSeats uint16, gameOptions uint32) []byte {
	// ZGameRoomMsgRoomInfo is variable-length. Minimum is the fixed header:
	// userID(4) + numTables(2) + numSeats(2) + gameOptions(4) + numPlayers(2) + numTableInfos(2) = 16 bytes
	b := make([]byte, 16)
	wire.WriteLE32(b[0:], userID)
	wire.WriteLE16(b[4:], numTables)
	wire.WriteLE16(b[6:], numSeats)
	wire.WriteLE32(b[8:], gameOptions)
	wire.WriteLE16(b[12:], 0) // numPlayers
	wire.WriteLE16(b[14:], 0) // numTableInfos
	return b
}
