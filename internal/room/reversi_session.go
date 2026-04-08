package room

import (
	"encoding/hex"
	"fmt"
	"log"

	"zone.com/internal/proto"
	"zone.com/internal/reversi"
)

const (
	reversiGameStateNotInited = 0
	reversiGameStateMove      = 1
	reversiGameStateGameOver  = 3
	reversiGameStateWaitNew   = 6
)

type reversiSession struct {
	game *reversi.Game
}

func newReversiSession() GameSession {
	return &reversiSession{game: reversi.NewGame()}
}

func (s *reversiSession) MessageName(t uint32) string {
	switch t {
	case proto.ReversiMsgNewGame:
		return "NewGame"
	case proto.ReversiMsgMovePiece:
		return "MovePiece"
	case proto.ReversiMsgTalk:
		return "Talk"
	case proto.ReversiMsgEndGame:
		return "EndGame"
	case proto.ReversiMsgEndLog:
		return "EndLog"
	case proto.ReversiMsgFinishMove:
		return "FinishMove"
	case proto.ReversiMsgPlayers:
		return "Players"
	case proto.ReversiMsgGameStateReq:
		return "GameStateReq"
	case proto.ReversiMsgGameStateResp:
		return "GameStateResp"
	case proto.ReversiMsgMoveTimeout:
		return "MoveTimeout"
	case proto.ReversiMsgVoteNewGame:
		return "VoteNewGame"
	default:
		return fmt.Sprintf("Unknown(0x%x)", t)
	}
}

func (s *reversiSession) HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error {
	switch msgType {
	case proto.ReversiMsgNewGame, proto.ReversiMsgPlayers:
		return s.handleNewGame(table, p, payload)
	case proto.ReversiMsgMovePiece:
		return s.handleMovePiece(table, p, payload)
	case proto.ReversiMsgFinishMove:
		return s.handleFinishMove(table, p, payload)
	case proto.ReversiMsgEndGame:
		return s.handleEndGame(table, p, payload)
	case proto.ReversiMsgTalk:
		table.BroadcastGameMsg(msgType, payload, -1)
	case proto.ReversiMsgGameStateReq:
		return s.handleGameStateReq(table, p, payload)
	case proto.ReversiMsgVoteNewGame:
		return s.handleVoteNewGame(table, p, payload)
	case proto.ReversiMsgEndLog:
		log.Printf("[reversi] player %d: EndLog received, suppressing relay", p.UserID)
	default:
		log.Printf("[reversi] player %d: UNHANDLED msg 0x%x(%s) payload=%s", p.UserID, msgType, s.MessageName(msgType), hex.EncodeToString(payload))
	}
	return nil
}

func (s *reversiSession) handleNewGame(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.ReversiNewGameMsgSize {
		return nil
	}
	var msg proto.ReversiNewGame
	msg.Unmarshal(payload)
	wasRematchVote := s.clientGameState() == reversiGameStateGameOver || s.clientGameState() == reversiGameStateWaitNew
	if wasRematchVote {
		vote := proto.ReversiVoteNewGame{Seat: p.Seat}
		table.BroadcastGameMsg(proto.ReversiMsgVoteNewGame, vote.Marshal(), -1)
	}
	resp := proto.ReversiNewGame{
		ProtocolSignature: int32(proto.ReversiSig),
		ProtocolVersion:   int32(proto.ReversiVersion),
		ClientVersion:     msg.ClientVersion,
		PlayerID:          p.UserID,
		Seat:              p.Seat,
	}
	table.BroadcastGameMsg(proto.ReversiMsgNewGame, resp.Marshal(), -1)
	if s.game.VoteNewGame(p.Seat) {
		s.game.Reset()
	}
	return nil
}

func (s *reversiSession) handleMovePiece(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.ReversiMovePieceSize {
		return nil
	}
	var msg proto.ReversiMovePiece
	msg.Unmarshal(payload)
	if err := s.game.HandleMove(p.Seat, msg.Col, msg.Row); err != nil {
		log.Printf("[reversi] player %d: invalid move: %v", p.UserID, err)
		return nil
	}
	// XP Reversi clients no longer exchange FinishMove as the normal network turn
	// transition. Advance turn state as soon as a move is accepted so the next
	// player stays in sync with the server.
	s.game.HandleFinishMove()
	table.BroadcastGameMsg(proto.ReversiMsgMovePiece, payload, -1)
	return nil
}

func (s *reversiSession) handleFinishMove(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.ReversiFinishMoveSize {
		return nil
	}
	// Legacy compatibility only. The XP client removed FinishMove from the
	// ordinary move flow, so applying it here would double-advance turns.
	log.Printf("[reversi] player %d: FinishMove received, suppressing relay", p.UserID)
	return nil
}

func (s *reversiSession) handleEndGame(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.ReversiEndGameSize {
		return nil
	}
	var msg proto.ReversiEndGame
	msg.Unmarshal(payload)
	s.game.HandleEndGame(msg.Flags)
	table.BroadcastGameMsg(proto.ReversiMsgEndGame, payload, -1)
	return nil
}

func (s *reversiSession) handleGameStateReq(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.ReversiGameStateReqSize {
		return nil
	}
	var req proto.ReversiGameStateReq
	req.Unmarshal(payload)
	resp := proto.ReversiGameStateResp{
		UserID:      req.UserID,
		Seat:        req.Seat,
		GameState:   s.clientGameState(),
		NewGameVote: s.game.NewGameVotes,
		FinalScore:  s.game.FinalScore,
		WhiteScore:  s.game.WhiteScore(),
		BlackScore:  s.game.BlackScore(),
		State:       s.game.Serialize(),
	}
	for i, player := range table.Seats {
		if player == nil {
			continue
		}
		resp.Players[i] = proto.ReversiPlayerInfo{UserID: player.UserID, Name: player.UserName}
	}
	table.SendToSeat(p.Seat, proto.ReversiMsgGameStateResp, resp.Marshal())
	return nil
}

func (s *reversiSession) handleVoteNewGame(table *Table, p *Player, payload []byte) error {
	table.BroadcastGameMsg(proto.ReversiMsgVoteNewGame, payload, -1)
	return nil
}

func (s *reversiSession) clientGameState() int16 {
	switch s.game.State {
	case reversi.StateGameOver:
		if s.game.NewGameVotes[0] || s.game.NewGameVotes[1] {
			return reversiGameStateWaitNew
		}
		return reversiGameStateGameOver
	case reversi.StatePlaying:
		return reversiGameStateMove
	default:
		return reversiGameStateNotInited
	}
}
