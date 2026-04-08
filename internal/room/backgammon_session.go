package room

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"zone.com/internal/backgammon"
	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

type backgammonSession struct {
	mu          sync.Mutex
	state       *backgammon.SharedState
	checkedIn   [2]bool
	checkIns    [2]proto.GameCheckIn
	delivered   [2][2]bool
	readyVotes  [2]bool
	clientVer   uint32
	initialRoll [2]int16
}

func newBackgammonSession() GameSession {
	return &backgammonSession{
		state:       backgammon.NewSharedState(),
		initialRoll: [2]int16{-1, -1},
	}
}

func (s *backgammonSession) MessageName(t uint32) string {
	switch t {
	case proto.GameMsgCheckIn:
		return "CheckIn"
	case proto.GameMsgGameStateRequest:
		return "GameStateRequest"
	case proto.GameMsgGameStateResponse:
		return "GameStateResponse"
	case proto.BackgammonMsgTalk:
		return "Talk"
	case proto.BackgammonMsgTransaction:
		return "Transaction"
	case proto.BackgammonMsgTimestamp:
		return "Timestamp"
	case proto.BackgammonMsgSavedGameState:
		return "SavedGameState"
	case proto.BackgammonMsgRollRequest:
		return "RollRequest"
	case proto.BackgammonMsgDiceRoll:
		return "DiceRoll"
	case proto.BackgammonMsgEndLog:
		return "EndLog"
	case proto.BackgammonMsgNewMatch:
		return "NewMatch"
	case proto.BackgammonMsgFirstMove:
		return "FirstMove"
	case proto.BackgammonMsgMoveTimeout:
		return "MoveTimeout"
	case proto.BackgammonMsgEndTurn:
		return "EndTurn"
	case proto.BackgammonMsgEndGame:
		return "EndGame"
	case proto.BackgammonMsgGoFirstRoll:
		return "GoFirstRoll"
	case proto.BackgammonMsgTieRoll:
		return "TieRoll"
	case proto.BackgammonMsgCheater:
		return "Cheater"
	default:
		return fmt.Sprintf("Unknown(0x%x)", t)
	}
}

func backgammonStateTagName(tag int32) string {
	switch tag {
	case backgammon.StateTagBGState:
		return "BGState"
	case backgammon.StateTagCrawford:
		return "Crawford"
	case backgammon.StateTagTimestampHi:
		return "TimestampHi"
	case backgammon.StateTagTimestampLo:
		return "TimestampLo"
	case backgammon.StateTagTimestampSet:
		return "TimestampSet"
	case backgammon.StateTagSettingsReady:
		return "SettingsReady"
	case backgammon.StateTagGameOverReason:
		return "GameOverReason"
	case backgammon.StateTagUserIDs:
		return "UserIDs"
	case backgammon.StateTagActiveSeat:
		return "ActiveSeat"
	case backgammon.StateTagAutoDouble:
		return "AutoDouble"
	case backgammon.StateTagHostBrown:
		return "HostBrown"
	case backgammon.StateTagTargetScore:
		return "TargetScore"
	case backgammon.StateTagSettingsDone:
		return "SettingsDone"
	case backgammon.StateTagCubeValue:
		return "CubeValue"
	case backgammon.StateTagCubeOwner:
		return "CubeOwner"
	case backgammon.StateTagResignPoints:
		return "ResignPoints"
	case backgammon.StateTagScore:
		return "Score"
	case backgammon.StateTagAllowWatching:
		return "AllowWatching"
	case backgammon.StateTagSilenceKibitzers:
		return "SilenceKibitzers"
	case backgammon.StateTagDice:
		return "Dice"
	case backgammon.StateTagDiceSize:
		return "DiceSize"
	case backgammon.StateTagReady:
		return "Ready"
	case backgammon.StateTagPieces:
		return "Pieces"
	default:
		return fmt.Sprintf("Tag(%d)", tag)
	}
}

func backgammonTransTagName(tag int32) string {
	switch tag {
	case backgammon.TransStateChange:
		return "StateChange"
	case backgammon.TransInitSettings:
		return "InitSettings"
	case backgammon.TransDice:
		return "Dice"
	case backgammon.TransDoublingCube:
		return "DoublingCube"
	case backgammon.TransBoard:
		return "Board"
	case backgammon.TransAcceptDouble:
		return "AcceptDouble"
	case backgammon.TransAllowWatchers:
		return "AllowWatchers"
	case backgammon.TransSilenceKibitzers:
		return "SilenceKibitzers"
	case backgammon.TransSettingsDlgReady:
		return "SettingsDlgReady"
	case backgammon.TransTimestamp:
		return "Timestamp"
	case backgammon.TransRestoreGame:
		return "RestoreGame"
	case backgammon.TransMiss:
		return "Miss"
	case backgammon.TransReady:
		return "Ready"
	default:
		return fmt.Sprintf("Trans(%d)", tag)
	}
}

func backgammonStateName(state int32) string {
	switch state {
	case backgammon.StateNotInit:
		return "NotInit"
	case backgammon.StateWaitingForGame:
		return "WaitingForGame"
	case backgammon.StateCheckSavedGame:
		return "CheckSavedGame"
	case backgammon.StateRestoreSavedGame:
		return "RestoreSavedGame"
	case backgammon.StateGameSettings:
		return "GameSettings"
	case backgammon.StateInitialRoll:
		return "InitialRoll"
	case backgammon.StateDouble:
		return "Double"
	case backgammon.StateRoll:
		return "Roll"
	case backgammon.StateRollPostDouble:
		return "RollPostDouble"
	case backgammon.StateRollPostResign:
		return "RollPostResign"
	case backgammon.StateMove:
		return "Move"
	case backgammon.StateEndTurn:
		return "EndTurn"
	case backgammon.StateGameOver:
		return "GameOver"
	case backgammon.StateMatchOver:
		return "MatchOver"
	case backgammon.StateNewMatch:
		return "NewMatch"
	case backgammon.StateDelete:
		return "Delete"
	case backgammon.StateResignOffer:
		return "ResignOffer"
	case backgammon.StateResignAccept:
		return "ResignAccept"
	case backgammon.StateResignRefused:
		return "ResignRefused"
	default:
		return fmt.Sprintf("State(%d)", state)
	}
}

func (s *backgammonSession) HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msgType {
	case proto.GameMsgCheckIn:
		return s.handleCheckIn(table, p, payload)
	case proto.GameMsgGameStateRequest:
		return s.handleGameStateRequest(table, p, payload)
	case proto.BackgammonMsgTalk:
		return s.handleTalk(table, p, payload)
	case proto.BackgammonMsgTransaction:
		return s.handleTransaction(table, p, payload)
	case proto.BackgammonMsgRollRequest:
		return s.handleRollRequest(table, p, payload)
	case proto.BackgammonMsgNewMatch:
		return s.handleNewMatch(table, p)
	case proto.BackgammonMsgTimestamp:
		log.Printf("[backgammon] player %d: relaying %s to opponent", p.UserID, s.MessageName(msgType))
		table.BroadcastGameMsg(msgType, payload, p.Seat)
	case proto.BackgammonMsgEndGame, proto.BackgammonMsgEndLog,
		proto.BackgammonMsgTieRoll, proto.BackgammonMsgMoveTimeout,
		proto.BackgammonMsgCheater, proto.BackgammonMsgEndTurn,
		proto.BackgammonMsgFirstMove, proto.BackgammonMsgGoFirstRoll:
		// XP client has no handler for these; relaying triggers ErrorTextSync.
		log.Printf("[backgammon] player %d: %s consumed server-side", p.UserID, s.MessageName(msgType))
	default:
		log.Printf("[backgammon] player %d: UNHANDLED msg 0x%x(%s) payload=%s",
			p.UserID, msgType, s.MessageName(msgType), hex.EncodeToString(payload))
	}
	return nil
}

func (s *backgammonSession) handleCheckIn(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.BackgammonCheckInSize {
		return nil
	}
	var msg proto.GameCheckIn
	msg.Unmarshal(payload)
	if msg.ProtocolSignature != backgammon.ProtocolSignature || msg.ProtocolVersion < backgammon.ProtocolVersion || msg.Seat != p.Seat {
		log.Printf("[backgammon] player %d: invalid CheckIn sig=%08x ver=%d seat=%d", p.UserID, msg.ProtocolSignature, msg.ProtocolVersion, msg.Seat)
		return nil
	}
	s.clientVer = msg.ClientVersion
	s.checkedIn[p.Seat] = true
	s.state.Set(backgammon.StateTagUserIDs, int32(p.Seat), int32(p.UserID))
	resp := proto.GameCheckIn{
		ProtocolSignature: backgammon.ProtocolSignature,
		ProtocolVersion:   backgammon.ProtocolVersion,
		ClientVersion:     msg.ClientVersion,
		PlayerID:          p.UserID,
		Seat:              p.Seat,
		PlayerType:        msg.PlayerType,
	}
	s.checkIns[p.Seat] = resp
	// Self-echo: client needs m_CheckIn set for ALL seats before proceeding.
	table.SendToSeat(p.Seat, proto.GameMsgCheckIn, resp.Marshal())
	oppSeat := p.Seat ^ 1
	if s.checkedIn[oppSeat] && !s.delivered[p.Seat][oppSeat] {
		table.SendToSeat(p.Seat, proto.GameMsgCheckIn, s.checkIns[oppSeat].Marshal())
		s.delivered[p.Seat][oppSeat] = true
	}
	if table.Seats[oppSeat] != nil && !s.delivered[oppSeat][p.Seat] {
		table.SendToSeat(oppSeat, proto.GameMsgCheckIn, resp.Marshal())
		s.delivered[oppSeat][p.Seat] = true
	}
	return nil
}

func (s *backgammonSession) handleTalk(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.BackgammonTalkHdrSize {
		return nil
	}
	var talk proto.BackgammonTalk
	talk.Unmarshal(payload)
	if talk.UserID != p.UserID {
		header := proto.BackgammonTalk{UserID: p.UserID, Seat: p.Seat, MessageLen: talk.MessageLen}
		fixed := append(header.Marshal(), payload[proto.BackgammonTalkHdrSize:]...)
		table.BroadcastGameMsg(proto.BackgammonMsgTalk, fixed, -1)
		return nil
	}
	table.BroadcastGameMsg(proto.BackgammonMsgTalk, payload, -1)
	return nil
}

func (s *backgammonSession) handleGameStateRequest(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.BackgammonGameStateReqSize {
		return nil
	}
	var req proto.GameStateRequest
	req.Unmarshal(payload)
	resp := proto.GameStateResponse{PlayerID: req.PlayerID, Seat: p.Seat}
	dump := s.state.Dump()
	hdr := resp.Marshal()
	buf := make([]byte, len(hdr)+4+len(dump)+4)
	copy(buf, hdr)
	off := len(hdr)
	wire.WriteLE32(buf[off:], uint32(len(dump)))
	off += 4
	copy(buf[off:], dump)
	off += len(dump)
	wire.WriteLE32(buf[off:], 0)
	table.SendToSeat(p.Seat, proto.GameMsgGameStateResponse, buf)
	return nil
}

func (s *backgammonSession) handleTransaction(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.BackgammonTransactionHdrSize {
		return nil
	}
	var tx proto.BackgammonTransaction
	tx.Unmarshal(payload)
	if tx.User != p.UserID || tx.Seat != int32(p.Seat) {
		log.Printf("[backgammon] player %d: invalid transaction header user=%d seat=%d", p.UserID, tx.User, tx.Seat)
		return nil
	}
	if !s.validateTransaction(tx) {
		log.Printf("[backgammon] player %d: rejected malformed transaction tag=%d cnt=%d items=%d",
			p.UserID, tx.TransTag, tx.TransCnt, len(tx.Items))
		return nil
	}
	if tx.TransTag == backgammon.TransReady {
		s.readyVotes[p.Seat] = true
	}
	for _, item := range tx.Items {
		s.state.Apply(item.EntryTag, item.EntryIdx, item.EntryVal)
	}
	log.Printf("[backgammon] player %d seat=%d: relaying tx %s cnt=%d state=%s active=%d",
		p.UserID,
		p.Seat,
		backgammonTransTagName(tx.TransTag),
		tx.TransCnt,
		backgammonStateName(s.state.Get(backgammon.StateTagBGState, 0)),
		s.state.Get(backgammon.StateTagActiveSeat, 0),
	)
	for i, item := range tx.Items {
		if item.EntryTag == backgammon.StateTagBGState {
			log.Printf("[backgammon]   item[%d]: %s=%d(%s)", i, backgammonStateTagName(item.EntryTag), item.EntryVal, backgammonStateName(item.EntryVal))
			continue
		}
		log.Printf("[backgammon]   item[%d]: %s[%d]=%d", i, backgammonStateTagName(item.EntryTag), item.EntryIdx, item.EntryVal)
	}
	for seat, pl := range table.Seats {
		if pl == nil || int16(seat) == p.Seat {
			continue
		}
		table.SendToSeat(int16(seat), proto.BackgammonMsgTransaction, payload)
	}
	return nil
}

func (s *backgammonSession) validateTransaction(tx proto.BackgammonTransaction) bool {
	if tx.TransCnt < 0 || int(tx.TransCnt) != len(tx.Items) {
		return false
	}
	if tx.TransTag < 0 || tx.TransTag > backgammon.TransReady {
		return false
	}
	for _, item := range tx.Items {
		if item.EntryTag < 0 || int(item.EntryTag) >= len(backgammon.SharedStateCounts) {
			return false
		}
		count := backgammon.SharedStateCounts[item.EntryTag]
		if count <= 1 {
			if item.EntryIdx != -1 {
				return false
			}
			continue
		}
		if item.EntryIdx < 0 || int(item.EntryIdx) >= count {
			return false
		}
	}
	return true
}

func (s *backgammonSession) handleRollRequest(table *Table, p *Player, payload []byte) error {
	if len(payload) < proto.BackgammonRollRequestSize {
		return nil
	}
	var req proto.BackgammonRollRequest
	req.Unmarshal(payload)
	if req.Seat != p.Seat {
		return nil
	}
	state := s.state.Get(backgammon.StateTagBGState, 0)
	d1, d2 := s.rollForState(state)
	log.Printf("[backgammon] player %d seat=%d: RollRequest state=%s active=%d -> d1=%d uses=%d d2=%d uses=%d",
		p.UserID,
		p.Seat,
		backgammonStateName(state),
		s.state.Get(backgammon.StateTagActiveSeat, 0),
		d1.Value, d1.NumUses,
		d2.Value, d2.NumUses,
	)
	msg := proto.BackgammonDiceRoll{
		Seat: p.Seat,
		D1:   d1,
		D2:   d2,
	}
	if state == backgammon.StateInitialRoll {
		s.initialRoll[p.Seat] = d1.Value
	}
	table.BroadcastGameMsg(proto.BackgammonMsgDiceRoll, msg.Marshal(), -1)
	return nil
}

func (s *backgammonSession) rollForState(state int32) (proto.BackgammonDiceInfo, proto.BackgammonDiceInfo) {
	d1v := backgammon.RollDie()
	d2v := backgammon.RollDie()
	d1 := backgammon.EncodeDice(d1v)
	d2 := backgammon.EncodeDice(d2v)
	if d1v == d2v {
		backgammon.EncodeUses(&d1, 2)
		backgammon.EncodeUses(&d2, 2)
	} else {
		backgammon.EncodeUses(&d1, 1)
		backgammon.EncodeUses(&d2, 1)
	}
	return toProtoDice(d1), toProtoDice(d2)
}

func (s *backgammonSession) handleNewMatch(table *Table, p *Player) error {
	s.state.ResetForNewMatch()
	s.readyVotes = [2]bool{}
	s.initialRoll = [2]int16{-1, -1}
	s.delivered = [2][2]bool{}
	for i, pl := range table.Seats {
		if pl == nil {
			continue
		}
		s.state.Set(backgammon.StateTagUserIDs, int32(i), int32(pl.UserID))
	}
	return nil
}

func toProtoDice(d backgammon.DiceInfo) proto.BackgammonDiceInfo {
	return proto.BackgammonDiceInfo{
		Value:        d.Value,
		EncodedValue: d.EncodedValue,
		EncoderMul:   d.EncoderMul,
		EncoderAdd:   d.EncoderAdd,
		NumUses:      d.NumUses,
	}
}

