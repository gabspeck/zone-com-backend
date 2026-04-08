package room

import (
	"encoding/hex"
	"fmt"
	"log"

	"zone.com/internal/checkers"
	"zone.com/internal/proto"
)

type checkersSession struct {
	game *checkers.Game
}

func newCheckersSession() GameSession {
	return &checkersSession{game: checkers.NewGame()}
}

func (s *checkersSession) MessageName(t uint32) string {
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

func (s *checkersSession) HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error {
	game := s.game
	switch msgType {
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
		table.BroadcastGameMsg(msgType, payload, -1)
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
	default:
		log.Printf("[game] player %d: UNHANDLED checkers msg 0x%x(%s) payload=%s",
			p.UserID, msgType, s.MessageName(msgType), hex.EncodeToString(payload))
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

	resp := proto.CheckersNewGame{
		ProtocolSignature: int32(proto.CheckersSig),
		ProtocolVersion:   int32(proto.CheckersVersion),
		ClientVersion:     msg.ClientVersion,
		PlayerID:          p.UserID,
		Seat:              p.Seat,
	}
	respData := resp.Marshal()
	table.BroadcastGameMsg(proto.CheckersMsgNewGame, respData, -1)

	log.Printf("[checkers] table %d: NewGame votes before: [%v, %v]", table.ID, game.NewGameVotes[0], game.NewGameVotes[1])
	if game.VoteNewGame(p.Seat) {
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
		p.UserID, p.Seat, msg.StartCol, msg.StartRow, msg.FinishCol, msg.FinishRow,
		squareName(msg.StartCol, msg.StartRow), squareName(msg.FinishCol, msg.FinishRow))
	log.Printf("[checkers] table %d: board BEFORE move:\n%s", table.ID, game.Board.String())
	flags, err := game.HandleMovePiece(p.Seat, msg.StartCol, msg.StartRow, msg.FinishCol, msg.FinishRow)
	if err != nil {
		log.Printf("[checkers] player %d: INVALID move: %v", p.UserID, err)
		return nil
	}
	log.Printf("[checkers] player %d: move OK, flags=%08x (%s)", p.UserID, flags, flagsString(flags))
	log.Printf("[checkers] table %d: board AFTER move:\n%s", table.ID, game.Board.String())
	table.BroadcastGameMsg(proto.CheckersMsgMovePiece, payload, -1)
	return nil
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
	flags, err := game.HandleFinishMove(p.Seat)
	if err != nil {
		log.Printf("[checkers] player %d: FinishMove ERROR: %v", p.UserID, err)
		return nil
	}
	log.Printf("[checkers] table %d: after FinishMove: playerToMove=%d moveCount=%d flags=%08x(%s) state=%d",
		table.ID, game.PlayerToMove, game.MoveCount, flags, flagsString(flags), game.State)
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
	game.HandleDraw(msg.Seat, msg.Vote)
	table.BroadcastGameMsg(proto.CheckersMsgDraw, payload, -1)
	return nil
}

func handleCheckersVoteNewGame(table *Table, game *checkers.Game, p *Player, payload []byte) error {
	if len(payload) < proto.CheckersVoteNewGameSize {
		log.Printf("[checkers] player %d: VoteNewGame too short (%d < %d)", p.UserID, len(payload), proto.CheckersVoteNewGameSize)
		return nil
	}
	table.BroadcastGameMsg(proto.CheckersMsgVoteNewGame, payload, -1)
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
	return fmt.Sprintf("%v", parts)
}
