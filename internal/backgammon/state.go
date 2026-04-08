package backgammon

import "math/rand"

const (
	ProtocolSignature uint32 = 0x42434B47 // 'BCKG'
	ProtocolVersion   uint32 = 3

	StateNotInit          = 0
	StateWaitingForGame   = 1
	StateCheckSavedGame   = 2
	StateRestoreSavedGame = 3
	StateGameSettings     = 4
	StateInitialRoll      = 5
	StateDouble           = 6
	StateRoll             = 7
	StateRollPostDouble   = 8
	StateRollPostResign   = 9
	StateMove             = 10
	StateEndTurn          = 11
	StateGameOver         = 12
	StateMatchOver        = 13
	StateNewMatch         = 14
	StateDelete           = 15
	StateResignOffer      = 16
	StateResignAccept     = 17
	StateResignRefused    = 18

	StateTagBGState          = 0
	StateTagCrawford         = 1
	StateTagTimestampHi      = 2
	StateTagTimestampLo      = 3
	StateTagTimestampSet     = 4
	StateTagSettingsReady    = 5
	StateTagGameOverReason   = 6
	StateTagUserIDs          = 7
	StateTagActiveSeat       = 8
	StateTagAutoDouble       = 9
	StateTagHostBrown        = 10
	StateTagTargetScore      = 11
	StateTagSettingsDone     = 12
	StateTagCubeValue        = 13
	StateTagCubeOwner        = 14
	StateTagResignPoints     = 15
	StateTagScore            = 16
	StateTagAllowWatching    = 17
	StateTagSilenceKibitzers = 18
	StateTagDice             = 19
	StateTagDiceSize         = 20
	StateTagReady            = 21
	StateTagPieces           = 22

	TransStateChange      = 0
	TransInitSettings     = 1
	TransDice             = 2
	TransDoublingCube     = 3
	TransBoard            = 4
	TransAcceptDouble     = 5
	TransAllowWatchers    = 6
	TransSilenceKibitzers = 7
	TransSettingsDlgReady = 8
	TransTimestamp        = 9
	TransRestoreGame      = 10
	TransMiss             = 11
	TransReady            = 12
)

var SharedStateCounts = []int{
	1, 1, 2, 2, 2, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 4, 4, 1, 30,
}

var InitPiecePositions = []int32{
	5, 5, 5, 5, 5, 7, 7, 7, 12, 12, 12, 12, 12, 23, 23,
	0, 0, 11, 11, 11, 11, 11, 16, 16, 16, 18, 18, 18, 18, 18,
}

type DiceInfo struct {
	Value        int16
	EncodedValue int32
	EncoderMul   int16
	EncoderAdd   int16
	NumUses      int32
}

type SharedState struct {
	entries [][]int32
}

func NewSharedState() *SharedState {
	s := &SharedState{entries: make([][]int32, len(SharedStateCounts))}
	for i, n := range SharedStateCounts {
		s.entries[i] = make([]int32, n)
		for j := range s.entries[i] {
			s.entries[i][j] = -1
		}
	}
	s.ResetForNewMatch()
	return s
}

func (s *SharedState) ResetForNewMatch() {
	for i := range s.entries {
		for j := range s.entries[i] {
			s.entries[i][j] = -1
		}
	}
	s.entries[StateTagBGState][0] = StateNotInit
	s.entries[StateTagActiveSeat][0] = 0
	s.entries[StateTagCubeValue][0] = 1
	s.entries[StateTagCubeOwner][0] = 0
	s.entries[StateTagReady][0] = 0
	s.entries[StateTagCrawford][0] = -1
	s.entries[StateTagHostBrown][0] = 1
	s.entries[StateTagTargetScore][0] = 3
	s.entries[StateTagAutoDouble][0] = 0
	s.entries[StateTagAllowWatching][0] = 1
	s.entries[StateTagAllowWatching][1] = 1
	s.entries[StateTagSilenceKibitzers][0] = 0
	s.entries[StateTagSilenceKibitzers][1] = 0
	s.ResetBoard()
	s.ResetDice(-1)
}

func (s *SharedState) ResetBoard() {
	copy(s.entries[StateTagPieces], InitPiecePositions)
}

func (s *SharedState) ResetDice(v int32) {
	for i := 0; i < 4; i++ {
		s.entries[StateTagDice][i] = v
		s.entries[StateTagDiceSize][i] = 0
	}
}

func (s *SharedState) Get(tag int32, idx int32) int32 {
	if tag < 0 || int(tag) >= len(s.entries) || idx < 0 || int(idx) >= len(s.entries[tag]) {
		return -1
	}
	return s.entries[tag][idx]
}

func (s *SharedState) Set(tag int32, idx int32, val int32) bool {
	if tag < 0 || int(tag) >= len(s.entries) || idx < 0 || int(idx) >= len(s.entries[tag]) {
		return false
	}
	s.entries[tag][idx] = val
	return true
}

func (s *SharedState) Dump() []byte {
	out := make([]byte, 0, s.Size())
	for _, entry := range s.entries {
		for _, v := range entry {
			out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
		}
	}
	return out
}

func (s *SharedState) Size() int {
	total := 0
	for _, entry := range s.entries {
		total += len(entry) * 4
	}
	return total
}

func (s *SharedState) Apply(tag, idx, val int32) bool {
	if idx < 0 {
		return s.Set(tag, 0, val)
	}
	return s.Set(tag, idx, val)
}

func EncodeDice(v int16) DiceInfo {
	d := DiceInfo{
		Value:      v,
		EncoderMul: int16(rand.Intn(1123) + 37),
		EncoderAdd: int16(rand.Intn(1263) + 183),
	}
	d.EncodedValue = (((int32(v)*int32(d.EncoderMul))+int32(d.EncoderAdd))*384 + 47)
	return d
}

func EncodeUses(d *DiceInfo, uses int32) {
	d.NumUses = (((uses * 16) + 31) * int32(d.EncoderMul+3)) + int32(d.EncoderAdd+4)
}

func RollDie() int16 {
	return int16(rand.Intn(6) + 1)
}
