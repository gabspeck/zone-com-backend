package room

import (
	"encoding/hex"
	"fmt"
	"log"

	"zone.com/internal/checkers"
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

func checkersMsgName(t uint32) string {
	switch t {
	case proto.CheckersMsgNewGame:
		return "NewGame"
	case proto.CheckersMsgMovePiece:
		return "MovePiece"
	case proto.CheckersMsgTalk:
		return "Talk"
	case proto.CheckersMsgEndGame:
		return "EndGame"
	case proto.CheckersMsgEndLog:
		return "EndLog"
	case proto.CheckersMsgFinishMove:
		return "FinishMove"
	case proto.CheckersMsgDraw:
		return "Draw"
	case proto.CheckersMsgPlayers:
		return "Players"
	case proto.CheckersMsgGameStateReq:
		return "GameStateReq"
	case proto.CheckersMsgGameStateResp:
		return "GameStateResp"
	case proto.CheckersMsgMoveTimeout:
		return "MoveTimeout"
	case proto.CheckersMsgVoteNewGame:
		return "VoteNewGame"
	default:
		return fmt.Sprintf("Unknown(0x%x)", t)
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
		if table.BothSeated() {
			gameID := rm.NextGameID()
			log.Printf("[seat] table %d: both seated -> starting game %d (seat0=%d seat1=%d)",
				table.ID, gameID, table.Seats[0].UserID, table.Seats[1].UserID)
			table.StartGame(gameID)

			// Send StartGame to both players
			for s := int16(0); s < 2; s++ {
				pl := table.Seats[s]
				if pl == nil {
					continue
				}
				startMsg := proto.MarshalRoomStartGame(gameID, table.ID, s)
				log.Printf("[seat] player %d: sending StartGame gameID=%d table=%d seat=%d data=%s",
					pl.UserID, gameID, table.ID, s, hex.EncodeToString(startMsg))
				if err := pl.Conn.SendAppMessage(proto.LobbySig, pl.Channel, proto.RoomMsgStartGame, startMsg); err != nil {
					log.Printf("[seat] player %d: send StartGame FAILED: %v", pl.UserID, err)
				}
			}
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
		p.UserID, gm.GameID, gm.MessageType, checkersMsgName(gm.MessageType), gm.MessageLen,
		hex.EncodeToString(payload))

	if p.Table == nil || p.Table.Game == nil {
		log.Printf("[game] player %d: game message but no table/game (table=%v)", p.UserID, p.Table)
		return nil
	}

	table := p.Table
	game := table.Game

	log.Printf("[game] player %d: on table %d, game state=%d, playerToMove=%d",
		p.UserID, table.ID, game.State, game.PlayerToMove)

	switch gm.MessageType {
	case proto.CheckersMsgNewGame:
		return handleCheckersNewGame(table, game, p, payload)
	case proto.CheckersMsgMovePiece:
		return handleCheckersMovePiece(table, game, p, payload)
	case proto.CheckersMsgFinishMove:
		return handleCheckersFinishMove(table, game, p, payload)
	case proto.CheckersMsgEndGame:
		return handleCheckersEndGame(table, game, p, payload)
	case proto.CheckersMsgDraw:
		return handleCheckersDraw(table, game, p, payload)
	case proto.CheckersMsgTalk:
		log.Printf("[game] player %d: Talk message, relaying to all (%d bytes)", p.UserID, len(payload))
		if len(payload) >= proto.CheckersTalkHdrSize {
			var talk proto.CheckersTalk
			talk.Unmarshal(payload)
			log.Printf("[game] player %d: Talk: userID=%d seat=%d msgLen=%d",
				p.UserID, talk.UserID, talk.Seat, talk.MessageLen)
		}
		table.BroadcastGameMsg(proto.CheckersMsgTalk, payload, -1)
	case proto.CheckersMsgVoteNewGame:
		return handleCheckersVoteNewGame(table, game, p, payload)
	case proto.CheckersMsgEndLog:
		log.Printf("[game] player %d: EndLog received, suppressing relay", p.UserID)
		if len(payload) >= proto.CheckersEndLogSize {
			var el proto.CheckersEndLog
			el.Unmarshal(payload)
			log.Printf("[game] player %d: EndLog: reason=%d seatLosing=%d seatQuitting=%d",
				p.UserID, el.Reason, el.SeatLosing, el.SeatQuitting)
		}
		// The original XP client uses EndLog as a local cleanup/failure path.
		// Relaying it after a normal draw/resign completion causes the peer to
		// show a corrupt/cannot-continue error even though EndGame already
		// finalized the session.
	default:
		log.Printf("[game] player %d: UNHANDLED checkers msg 0x%x(%s) payload=%s",
			p.UserID, gm.MessageType, checkersMsgName(gm.MessageType), hex.EncodeToString(payload))
	}
	return nil
}

func handleCheckersNewGame(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersNewGameMsgSize {
		log.Printf("[checkers] player %d: NewGame too short (%d < %d)", p.UserID, len(payload), proto.CheckersNewGameMsgSize)
		return nil
	}
	var msg proto.CheckersNewGame
	msg.Unmarshal(payload)
	log.Printf("[checkers] player %d: NewGame check-in: protSig=%08x protVer=%d clientVer=%d playerID=%d seat=%d",
		p.UserID, uint32(msg.ProtocolSignature), msg.ProtocolVersion, msg.ClientVersion, msg.PlayerID, msg.Seat)

	// Build response with playerID filled in
	resp := proto.CheckersNewGame{
		ProtocolSignature: int32(proto.CheckersSig),
		ProtocolVersion:   int32(proto.CheckersVersion),
		ClientVersion:     msg.ClientVersion,
		PlayerID:          p.UserID,
		Seat:              p.Seat,
	}

	respData := resp.Marshal()
	log.Printf("[checkers] player %d: relaying NewGame response to both clients: playerID=%d seat=%d data=%s",
		p.UserID, p.UserID, p.Seat, hex.EncodeToString(respData))

	// Both clients need server-filled NewGame packets for seat 0 and seat 1 so their
	// local newGameVote[] state reaches "both ready" and the board initializes.
	table.BroadcastGameMsg(proto.CheckersMsgNewGame, respData, -1)

	// Track votes
	log.Printf("[checkers] table %d: NewGame votes before: [%v, %v]", table.ID, game.NewGameVotes[0], game.NewGameVotes[1])
	if game.VoteNewGame(p.Seat) {
		// Both players checked in - start the game
		game.Reset()
		log.Printf("[checkers] table %d: BOTH players checked in, game now ACTIVE", table.ID)
		log.Printf("[checkers] table %d: initial board:\n%s", table.ID, game.Board.String())
	} else {
		log.Printf("[checkers] table %d: waiting for other player's NewGame check-in (votes: [%v, %v])",
			table.ID, game.NewGameVotes[0], game.NewGameVotes[1])
	}

	return nil
}

func handleCheckersMovePiece(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersMovePieceSize {
		log.Printf("[checkers] player %d: MovePiece too short (%d < %d)", p.UserID, len(payload), proto.CheckersMovePieceSize)
		return nil
	}
	var msg proto.CheckersMovePiece
	msg.Unmarshal(payload)

	log.Printf("[checkers] player %d (seat %d): MovePiece (%d,%d)->(%d,%d) [%s -> %s]",
		p.UserID, p.Seat,
		msg.StartCol, msg.StartRow, msg.FinishCol, msg.FinishRow,
		squareName(msg.StartCol, msg.StartRow), squareName(msg.FinishCol, msg.FinishRow))
	log.Printf("[checkers] table %d: board BEFORE move:\n%s", table.ID, game.Board.String())

	flags, err := game.HandleMovePiece(p.Seat, msg.StartCol, msg.StartRow, msg.FinishCol, msg.FinishRow)
	if err != nil {
		log.Printf("[checkers] player %d: INVALID move: %v", p.UserID, err)
		return nil // don't disconnect for invalid moves
	}

	log.Printf("[checkers] player %d: move OK, flags=%08x (%s)", p.UserID, flags, flagsString(flags))
	log.Printf("[checkers] table %d: board AFTER move:\n%s", table.ID, game.Board.String())

	// Broadcast the move to both players
	log.Printf("[checkers] table %d: broadcasting MovePiece to both players", table.ID)
	table.BroadcastGameMsg(proto.CheckersMsgMovePiece, payload, -1)
	return nil
}

func squareName(col, row byte) string {
	return fmt.Sprintf("%c%d", 'a'+col, row+1)
}

func flagsString(flags uint32) string {
	if flags == 0 {
		return "none"
	}
	var parts []string
	if flags&checkers.FlagWasJump != 0 {
		parts = append(parts, "JUMP")
	}
	if flags&checkers.FlagContinueJump != 0 {
		parts = append(parts, "CONTINUE_JUMP")
	}
	if flags&checkers.FlagPromote != 0 {
		parts = append(parts, "PROMOTE")
	}
	if flags&checkers.FlagStalemate != 0 {
		parts = append(parts, "STALEMATE")
	}
	if flags&checkers.FlagResign != 0 {
		parts = append(parts, "RESIGN")
	}
	if flags&checkers.FlagDraw != 0 {
		parts = append(parts, "DRAW")
	}
	if flags&checkers.FlagTimeLoss != 0 {
		parts = append(parts, "TIME_LOSS")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%x", flags)
	}
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += "|"
		}
		s += p
	}
	return s
}

func handleCheckersFinishMove(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersFinishMoveMsgSize {
		log.Printf("[checkers] player %d: FinishMove too short (%d < %d)", p.UserID, len(payload), proto.CheckersFinishMoveMsgSize)
		return nil
	}
	var msg proto.CheckersFinishMove
	msg.Unmarshal(payload)

	log.Printf("[checkers] player %d (seat %d): FinishMove: drawSeat=%d time=%d piece=0x%02x data=%s",
		p.UserID, p.Seat, msg.DrawSeat, msg.Time, msg.Piece, hex.EncodeToString(payload))
	log.Printf("[checkers] table %d: before FinishMove: playerToMove=%d moveCount=%d moveOver=%v",
		table.ID, game.PlayerToMove, game.MoveCount, game.MoveOver)

	flags, err := game.HandleFinishMove(p.Seat)
	if err != nil {
		log.Printf("[checkers] player %d: FinishMove ERROR: %v", p.UserID, err)
		return nil
	}

	log.Printf("[checkers] table %d: after FinishMove: playerToMove=%d moveCount=%d flags=%08x(%s) state=%d",
		table.ID, game.PlayerToMove, game.MoveCount, flags, flagsString(flags), game.State)

	if game.State == checkers.StateGameOver {
		log.Printf("[checkers] table %d: GAME OVER! finalScore=%d", table.ID, game.FinalScore)
	}

	// Broadcast finish move to both players
	log.Printf("[checkers] table %d: broadcasting FinishMove", table.ID)
	table.BroadcastGameMsg(proto.CheckersMsgFinishMove, payload, -1)
	return nil
}

func handleCheckersEndGame(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersEndGameSize {
		log.Printf("[checkers] player %d: EndGame too short (%d < %d)", p.UserID, len(payload), proto.CheckersEndGameSize)
		return nil
	}
	var msg proto.CheckersEndGame
	msg.Unmarshal(payload)

	log.Printf("[checkers] player %d (seat %d): EndGame: flags=%08x(%s) data=%s",
		p.UserID, p.Seat, msg.Flags, flagsString(msg.Flags), hex.EncodeToString(payload))

	if err := game.HandleEndGame(p.Seat, msg.Flags); err != nil {
		log.Printf("[checkers] player %d: EndGame ERROR: %v", p.UserID, err)
	}

	log.Printf("[checkers] table %d: after EndGame: state=%d finalScore=%d", table.ID, game.State, game.FinalScore)
	log.Printf("[checkers] table %d: broadcasting EndGame", table.ID)

	// Broadcast to both players
	table.BroadcastGameMsg(proto.CheckersMsgEndGame, payload, -1)
	return nil
}

func handleCheckersDraw(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersDrawSize {
		log.Printf("[checkers] player %d: Draw too short (%d < %d)", p.UserID, len(payload), proto.CheckersDrawSize)
		return nil
	}
	var msg proto.CheckersDraw
	msg.Unmarshal(payload)

	voteStr := "UNKNOWN"
	switch msg.Vote {
	case int16(proto.AcceptDraw):
		voteStr = "ACCEPT"
	case int16(proto.RefuseDraw):
		voteStr = "REFUSE"
	case 0:
		voteStr = "OFFER"
	}
	log.Printf("[checkers] player %d (seat %d): Draw: seat=%d vote=%d(%s) data=%s",
		p.UserID, p.Seat, msg.Seat, msg.Vote, voteStr, hex.EncodeToString(payload))

	game.HandleDraw(msg.Seat, msg.Vote)

	log.Printf("[checkers] table %d: broadcasting Draw to both players", table.ID)
	// The XP client expects both sides to process the draw message. On accept,
	// seat 0 then emits the authoritative EndGame(DRAW) packet.
	table.BroadcastGameMsg(proto.CheckersMsgDraw, payload, -1)
	return nil
}

func handleCheckersVoteNewGame(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersVoteNewGameSize {
		log.Printf("[checkers] player %d: VoteNewGame too short (%d < %d)", p.UserID, len(payload), proto.CheckersVoteNewGameSize)
		return nil
	}
	var msg proto.CheckersVoteNewGame
	msg.Unmarshal(payload)

	log.Printf("[checkers] player %d (seat %d): VoteNewGame: seat=%d data=%s",
		p.UserID, p.Seat, msg.Seat, hex.EncodeToString(payload))
	log.Printf("[checkers] table %d: broadcasting VoteNewGame to both", table.ID)

	// Broadcast the vote to opponent
	table.BroadcastGameMsg(proto.CheckersMsgVoteNewGame, payload, -1)
	return nil
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
