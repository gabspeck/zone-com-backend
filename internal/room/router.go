package room

import (
	"encoding/hex"
	"fmt"
	"log"

	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// HandlePlayer runs the message loop for a connected player.
// Called as a goroutine after handshake and proxy negotiation.
func HandlePlayer(rm *Room, p *Player) {
	log.Printf("[router] player %d (%s): entering message loop (table=%v seat=%d)",
		p.UserID, p.UserName, tableID(p.Table), p.Seat)
	defer func() {
		// Notify the remaining player at room/game level. Do not drop their
		// underlying lobby socket, or the XP client surfaces this as a full
		// network disconnect and blocks on a reconnect prompt.
		if p.Table != nil {
			opp := p.Table.Opponent(p.Seat)
			if opp != nil {
				log.Printf("[router] player %d: disconnected, notifying opponent %d on table %d",
					p.UserID, opp.UserID, p.Table.ID)
				notifyOpponentLeft(p, opp)
			} else {
				log.Printf("[router] player %d: disconnected, no opponent on table %d", p.UserID, p.Table.ID)
			}
		} else {
			log.Printf("[router] player %d: disconnected, not at a table", p.UserID)
		}
		rm.RemovePlayer(p)
		p.Conn.Close()
		log.Printf("[router] player %d: removed and connection closed", p.UserID)
	}()

	for {
		msgs, err := p.Conn.ReadAppMessages()
		if err != nil {
			log.Printf("[router] player %d: read error: %v", p.UserID, err)
			return
		}

		for _, msg := range msgs {
			log.Printf("[router] player %d: recv sig=%08x type=%d ch=%d datalen=%d",
				p.UserID, msg.Signature, msg.Type, msg.Channel, len(msg.Data))
			if msg.Signature != proto.LobbySig {
				log.Printf("[router] player %d: UNEXPECTED sig %08x (want %08x 'lbby'), skipping",
					p.UserID, msg.Signature, proto.LobbySig)
				continue
			}
			log.Printf("[router] player %d: dispatching room msg type=%d (%s)",
				p.UserID, msg.Type, roomMsgName(msg.Type))
			if err := dispatchRoomMsg(rm, p, msg); err != nil {
				log.Printf("[router] player %d: dispatch error: %v", p.UserID, err)
				return
			}
		}
	}
}

func notifyOpponentLeft(p, opp *Player) {
	if p.Table == nil || opp == nil {
		return
	}

	service := marshalProxyServiceName(opp.Service)
	disconnect := proto.MarshalProxyServiceInfo(
		proto.ProxyServiceDisconnect,
		service,
		opp.Channel,
		proto.ProxyServiceAvailable|proto.ProxyServiceLocal,
		[4]byte{127, 0, 0, 1},
		proto.PortMillenniumProxy,
	)
	intake := proto.MarshalProxyServiceInfo(
		proto.ProxyServiceInfo,
		service,
		opp.Channel,
		proto.ProxyServiceAvailable|proto.ProxyServiceLocal|proto.ProxyServiceConnected,
		[4]byte{127, 0, 0, 1},
		proto.PortMillenniumProxy,
	)
	data := make([]byte, 0, len(disconnect)+len(intake))
	data = append(data, disconnect...)
	data = append(data, intake...)
	msg := wire.AppMessage{
		Signature: proto.ProxySig,
		Channel:   0,
		Type:      2,
		Data:      data,
	}
	log.Printf("[router] player %d: sending packed proxy disconnect to opponent %d: service=%q channel=%d",
		p.UserID, opp.UserID, opp.Service, opp.Channel)
	if err := opp.Conn.WriteAppMessages([]wire.AppMessage{msg}); err != nil {
		log.Printf("[router] player %d: send ServiceDisconnect to opponent %d FAILED: %v", p.UserID, opp.UserID, err)
	}
}

func marshalProxyServiceName(service string) [proto.InternalNameLen + 1]byte {
	var out [proto.InternalNameLen + 1]byte
	copy(out[:], []byte(service))
	return out
}

func tableID(t *Table) string {
	if t == nil {
		return "<none>"
	}
	return fmt.Sprintf("%d", t.ID)
}

func roomMsgName(t uint32) string {
	switch t {
	case proto.RoomMsgUserInfo:
		return "UserInfo"
	case proto.RoomMsgRoomInfo:
		return "RoomInfo"
	case proto.RoomMsgEnter:
		return "Enter"
	case proto.RoomMsgLeave:
		return "Leave"
	case proto.RoomMsgStartGame:
		return "StartGame"
	case proto.RoomMsgGameMessage:
		return "GameMessage"
	case proto.RoomMsgAccessed:
		return "Accessed"
	case proto.RoomMsgChatSwitch:
		return "ChatSwitch"
	case proto.RoomMsgSeatRequest:
		return "SeatRequest"
	case proto.RoomMsgSeatResponse:
		return "SeatResponse"
	case proto.RoomMsgPing:
		return "Ping"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

func dispatchRoomMsg(rm *Room, p *Player, msg wire.AppMessage) error {
	switch msg.Type {
	case proto.RoomMsgSeatRequest:
		return handleSeatRequest(rm, p, msg)
	case proto.RoomMsgGameMessage:
		return handleGameMessage(rm, p, msg)
	case proto.RoomMsgChatSwitch:
		return handleChatSwitch(rm, p, msg)
	case proto.RoomMsgPing:
		return handlePing(p, msg)
	default:
		log.Printf("[router] player %d: unhandled room msg type=%d (%s) data=%s",
			p.UserID, msg.Type, roomMsgName(msg.Type), hex.EncodeToString(msg.Data))
	}
	return nil
}

func handleChatSwitch(rm *Room, p *Player, msg wire.AppMessage) error {
	if len(msg.Data) < 5 {
		log.Printf("[chat] player %d: ChatSwitch too short (%d < 5)", p.UserID, len(msg.Data))
		return nil
	}

	var sw proto.RoomChatSwitch
	sw.Unmarshal(msg.Data)
	log.Printf("[chat] player %d: ChatSwitch userID=%d chat=%v data=%s",
		p.UserID, sw.UserID, sw.Chat, hex.EncodeToString(msg.Data))

	if sw.UserID != p.UserID {
		log.Printf("[chat] player %d: ChatSwitch userID mismatch (got %d), forcing authoritative userID %d",
			p.UserID, sw.UserID, p.UserID)
	}

	p.Chat = sw.Chat
	rm.BroadcastChatSwitch(p)
	return nil
}

func handleSeatRequest(rm *Room, p *Player, msg wire.AppMessage) error {
	if len(msg.Data) < proto.RoomSeatRequestSize {
		log.Printf("[seat] player %d: SeatRequest too short (%d < %d)", p.UserID, len(msg.Data), proto.RoomSeatRequestSize)
		return nil
	}
	var req proto.RoomSeatRequest
	req.Unmarshal(msg.Data)
	log.Printf("[seat] player %d: SeatRequest: action=%d(%s) table=%d seat=%d userID=%d data=%s",
		p.UserID, req.Action, seatActionName(req.Action), req.Table, req.Seat, req.UserID,
		hex.EncodeToString(msg.Data))

	switch req.Action {
	case proto.SeatActionSitDown, proto.SeatActionQuickJoin:
		log.Printf("[seat] player %d: looking for a seat (action=%s)...", p.UserID, seatActionName(req.Action))
		table, seat := rm.FindSeat(p)
		if table == nil {
			log.Printf("[seat] player %d: NO available table, sending Denied", p.UserID)
			resp := proto.MarshalRoomSeatResponse(p.UserID, 0, -1, -1, proto.SeatActionDenied)
			return p.Conn.SendAppMessage(proto.LobbySig, p.Channel, proto.RoomMsgSeatResponse, resp)
		}

		log.Printf("[seat] player %d: seated at table %d seat %d", p.UserID, table.ID, seat)

		// Send seat response
		resp := proto.MarshalRoomSeatResponse(p.UserID, 0, table.ID, seat, proto.SeatActionSitDown)
		log.Printf("[seat] player %d: sending SeatResponse: table=%d seat=%d data=%s",
			p.UserID, table.ID, seat, hex.EncodeToString(resp))
		if err := p.Conn.SendAppMessage(proto.LobbySig, p.Channel, proto.RoomMsgSeatResponse, resp); err != nil {
			return err
		}

		// If both players are seated, start the game
		if table.AllSeated() {
			gameID := rm.NextGameID()
			log.Printf("[seat] table %d: all %d players seated -> starting game %d",
				table.ID, len(table.Seats), gameID)
			table.StartGame(gameID)
			SendStartGameMessages(table, gameID)
			log.Printf("[seat] game %d started on table %d", gameID, table.ID)
		} else {
			log.Printf("[seat] player %d: waiting for opponent on table %d", p.UserID, table.ID)
		}

	case proto.SeatActionLeaveTable:
		log.Printf("[seat] player %d: LeaveTable (current table=%s)", p.UserID, tableID(p.Table))
		if p.Table != nil {
			opp := p.Table.Opponent(p.Seat)
			tblID := p.Table.ID
			p.Table.RemovePlayer(p)
			log.Printf("[seat] player %d: removed from table %d", p.UserID, tblID)
			if opp != nil {
				log.Printf("[seat] player %d: closing opponent %d connection", p.UserID, opp.UserID)
				opp.Conn.Close()
			}
		}

	default:
		log.Printf("[seat] player %d: unhandled seat action %d(%s)", p.UserID, req.Action, seatActionName(req.Action))
	}

	return nil
}

func seatActionName(a int16) string {
	switch a {
	case proto.SeatActionSitDown:
		return "SitDown"
	case proto.SeatActionLeaveTable:
		return "LeaveTable"
	case proto.SeatActionStartGame:
		return "StartGame"
	case proto.SeatActionReplacePlayer:
		return "ReplacePlayer"
	case proto.SeatActionAddKibitzer:
		return "AddKibitzer"
	case proto.SeatActionRemoveKibitzer:
		return "RemoveKibitzer"
	case proto.SeatActionJoin:
		return "Join"
	case proto.SeatActionQuickHost:
		return "QuickHost"
	case proto.SeatActionQuickJoin:
		return "QuickJoin"
	case proto.SeatActionDenied:
		return "Denied"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

func handleGameMessage(rm *Room, p *Player, msg wire.AppMessage) error {
	if len(msg.Data) < proto.RoomGameMessageHdr {
		log.Printf("[game] player %d: GameMessage too short (%d < %d)", p.UserID, len(msg.Data), proto.RoomGameMessageHdr)
		return nil
	}
	var gm proto.RoomGameMessage
	gm.Unmarshal(msg.Data)
	payload := msg.Data[proto.RoomGameMessageHdr:]

	log.Printf("[game] player %d: GameMessage: gameID=%d msgType=0x%x(%s) msgLen=%d payload=%s",
		p.UserID, gm.GameID, gm.MessageType, gameMessageName(p.Table, gm.MessageType), gm.MessageLen,
		hex.EncodeToString(payload))

	if p.Table == nil || p.Table.Session == nil {
		log.Printf("[game] player %d: game message but no table/session (table=%v)", p.UserID, p.Table)
		return nil
	}

	table := p.Table
	return table.Session.HandleMessage(table, p, gm.MessageType, payload)
}

func handlePing(p *Player, msg wire.AppMessage) error {
	if len(msg.Data) < proto.RoomPingSize {
		log.Printf("[ping] player %d: Ping too short (%d < %d)", p.UserID, len(msg.Data), proto.RoomPingSize)
		return nil
	}
	userID := wire.ReadLE32(msg.Data[0:])
	pingTime := wire.ReadLE32(msg.Data[4:])

	log.Printf("[ping] player %d: Ping received: userID=%d pingTime=%d", p.UserID, userID, pingTime)

	resp := proto.MarshalRoomPing(p.UserID, pingTime)
	log.Printf("[ping] player %d: sending Pong: userID=%d pingTime=%d", p.UserID, p.UserID, pingTime)
	return p.Conn.SendAppMessage(proto.LobbySig, p.Channel, proto.RoomMsgPing, resp)
}

func gameMessageName(table *Table, msgType uint32) string {
	if table == nil || table.Session == nil {
		return fmt.Sprintf("Unknown(0x%x)", msgType)
	}
	return table.Session.MessageName(msgType)
}

func SendStartGameMessages(table *Table, gameID uint32) {
	players := make([]proto.RoomStartGameMPlayer, len(table.Seats))
	for i, p := range table.Seats {
		if p == nil {
			continue
		}
		players[i] = proto.RoomStartGameMPlayer{
			UserID: p.UserID,
			LCID:   p.LCID,
			Chat:   p.Chat,
			Skill:  p.Skill,
		}
	}
	for s := int16(0); s < int16(len(table.Seats)); s++ {
		pl := table.Seats[s]
		if pl == nil {
			continue
		}
		startMsg := proto.MarshalRoomStartGameM(gameID, table.ID, s, players)
		log.Printf("[start] player %d: sending StartGameM: gameID=%d table=%d seat=%d data=%s",
			pl.UserID, gameID, table.ID, s, hex.EncodeToString(startMsg))
		if err := pl.Conn.SendAppMessage(proto.LobbySig, pl.Channel, proto.RoomMsgStartGameM, startMsg); err != nil {
			log.Printf("[start] player %d: send StartGameM FAILED: %v", pl.UserID, err)
		}
	}
}
