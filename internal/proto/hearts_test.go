package proto

import (
	"testing"

	"zone.com/internal/wire"
)

func TestHeartsStartGameSize(t *testing.T) {
	m := HeartsStartGame{
		NumCardsInHand:  13,
		NumCardsInPass:  3,
		NumPointsInGame: 100,
		Players:         [HeartsMaxNumPlayers]uint32{100, 101, 102, 103},
	}
	b := m.Marshal()
	if len(b) != HeartsStartGameSize {
		t.Fatalf("StartGame size = %d, want %d", len(b), HeartsStartGameSize)
	}
}

func TestHeartsStartHandSize(t *testing.T) {
	m := HeartsStartHand{PassDir: HeartsPassLeft}
	for i := 0; i < 13; i++ {
		m.Cards[i] = byte(i)
	}
	for i := 13; i < HeartsMaxNumCardsInHand; i++ {
		m.Cards[i] = HeartsCardNone
	}
	b := m.Marshal()
	if len(b) != HeartsStartHandSize {
		t.Fatalf("StartHand size = %d, want %d", len(b), HeartsStartHandSize)
	}
}

func TestHeartsPassCardsRoundTrip(t *testing.T) {
	orig := HeartsPassCards{Seat: 2, Pass: [HeartsMaxNumCardsInPass]byte{0, 13, 26, HeartsCardNone, HeartsCardNone}}
	b := orig.Marshal()
	if len(b) != HeartsPassCardsSize {
		t.Fatalf("PassCards size = %d, want %d", len(b), HeartsPassCardsSize)
	}
	var got HeartsPassCards
	got.Unmarshal(b)
	if got.Seat != orig.Seat {
		t.Errorf("Seat = %d, want %d", got.Seat, orig.Seat)
	}
	for i := 0; i < HeartsMaxNumCardsInPass; i++ {
		if got.Pass[i] != orig.Pass[i] {
			t.Errorf("Pass[%d] = %d, want %d", i, got.Pass[i], orig.Pass[i])
		}
	}
}

func TestHeartsPlayCardRoundTrip(t *testing.T) {
	orig := HeartsPlayCard{Seat: 3, Card: 36} // QS
	b := orig.Marshal()
	if len(b) != HeartsPlayCardSize {
		t.Fatalf("PlayCard size = %d, want %d", len(b), HeartsPlayCardSize)
	}
	var got HeartsPlayCard
	got.Unmarshal(b)
	if got.Seat != orig.Seat || got.Card != orig.Card {
		t.Errorf("got {%d, %d}, want {%d, %d}", got.Seat, got.Card, orig.Seat, orig.Card)
	}
}

func TestHeartsEndHandSize(t *testing.T) {
	m := HeartsEndHand{
		Score:     [HeartsMaxNumPlayers]int16{5, 8, 0, 13, 0, 0},
		RunPlayer: -1,
	}
	b := m.Marshal()
	if len(b) != HeartsEndHandSize {
		t.Fatalf("EndHand size = %d, want %d", len(b), HeartsEndHandSize)
	}
}

func TestHeartsClientReadyUnmarshal(t *testing.T) {
	m := HeartsClientReady{
		ProtocolSignature: HeartsSig,
		ProtocolVersion:   HeartsVersion,
		Version:           1,
		Seat:              2,
	}
	// Build wire bytes manually (BE)
	b := make([]byte, HeartsClientReadySize)
	wire.WriteBE32(b[0:], m.ProtocolSignature)
	wire.WriteBE32(b[4:], m.ProtocolVersion)
	wire.WriteBE32(b[8:], m.Version)
	wire.WriteBE16(b[12:], uint16(m.Seat))

	var got HeartsClientReady
	got.Unmarshal(b)
	if got.ProtocolSignature != HeartsSig {
		t.Errorf("sig = 0x%x, want 0x%x", got.ProtocolSignature, HeartsSig)
	}
	if got.ProtocolVersion != HeartsVersion {
		t.Errorf("ver = %d, want %d", got.ProtocolVersion, HeartsVersion)
	}
	if got.Seat != 2 {
		t.Errorf("seat = %d, want 2", got.Seat)
	}
}

func TestHeartsTalkRoundTrip(t *testing.T) {
	orig := HeartsTalk{UserID: 100, Seat: 1, MessageLen: 5}
	b := orig.Marshal()
	if len(b) != HeartsTalkHdrSize {
		t.Fatalf("Talk header size = %d, want %d", len(b), HeartsTalkHdrSize)
	}
	var got HeartsTalk
	got.Unmarshal(b)
	if got.UserID != orig.UserID || got.Seat != orig.Seat || got.MessageLen != orig.MessageLen {
		t.Errorf("got {%d, %d, %d}, want {%d, %d, %d}", got.UserID, got.Seat, got.MessageLen, orig.UserID, orig.Seat, orig.MessageLen)
	}
}
