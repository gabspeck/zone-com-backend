package proto

import (
	"testing"

	"zone.com/internal/wire"
)

func TestSpadesClientReadySize(t *testing.T) {
	b := make([]byte, SpadesClientReadySize)
	wire.WriteBE32(b[0:], SpadesSig)
	wire.WriteBE32(b[4:], SpadesVersion)
	wire.WriteBE32(b[8:], 0x00010000)
	wire.WriteBE32(b[12:], 100) // playerID
	wire.WriteBE16(b[16:], 2)   // seat
	var m SpadesClientReady
	m.Unmarshal(b)
	if m.ProtocolSignature != SpadesSig {
		t.Errorf("sig = 0x%x, want 0x%x", m.ProtocolSignature, SpadesSig)
	}
	if m.ProtocolVersion != SpadesVersion {
		t.Errorf("ver = %d, want %d", m.ProtocolVersion, SpadesVersion)
	}
	if m.PlayerID != 100 {
		t.Errorf("playerID = %d, want 100", m.PlayerID)
	}
	if m.Seat != 2 {
		t.Errorf("seat = %d, want 2", m.Seat)
	}
}

func TestSpadesStartGameSize(t *testing.T) {
	m := SpadesStartGame{
		Players:         [SpadesNumPlayers]uint32{100, 101, 102, 103},
		GameOptions:     0,
		NumPointsInGame: 500,
		MinPointsInGame: -200,
	}
	b := m.Marshal()
	if len(b) != SpadesStartGameSize {
		t.Fatalf("StartGame size = %d, want %d", len(b), SpadesStartGameSize)
	}
	// Verify players are BE
	if wire.ReadBE32(b[0:]) != 100 {
		t.Error("players[0] mismatch")
	}
	// numPointsInGame at offset 20 is BE
	if wire.ReadBE16(b[20:]) != 500 {
		t.Error("numPointsInGame mismatch")
	}
	// minPointsInGame at offset 22 is LE (not endian-converted)
	if int16(wire.ReadLE16(b[22:])) != -200 {
		t.Errorf("minPointsInGame = %d, want -200", int16(wire.ReadLE16(b[22:])))
	}
}

func TestSpadesStartBidSize(t *testing.T) {
	m := SpadesStartBid{BoardNumber: 3, Dealer: 1}
	for i := 0; i < SpadesNumCardsInHand; i++ {
		m.Hand[i] = byte(i * 4)
	}
	b := m.Marshal()
	if len(b) != SpadesStartBidSize {
		t.Fatalf("StartBid size = %d, want %d", len(b), SpadesStartBidSize)
	}
	if wire.ReadBE16(b[0:]) != 3 {
		t.Error("boardNumber mismatch")
	}
	if wire.ReadBE16(b[2:]) != 1 {
		t.Error("dealer mismatch")
	}
	if b[4] != 0 {
		t.Errorf("hand[0] = %d, want 0", b[4])
	}
}

func TestSpadesBidRoundTrip(t *testing.T) {
	orig := SpadesBid{Seat: 1, NextBidder: 2, Bid: 5}
	b := orig.Marshal()
	if len(b) != SpadesBidSize {
		t.Fatalf("Bid size = %d, want %d", len(b), SpadesBidSize)
	}
	var got SpadesBid
	got.Unmarshal(b)
	if got.Seat != 1 || got.NextBidder != 2 || got.Bid != 5 {
		t.Errorf("got seat=%d next=%d bid=%d", got.Seat, got.NextBidder, got.Bid)
	}
}

func TestSpadesPlayRoundTrip(t *testing.T) {
	orig := SpadesPlay{Seat: 3, NextPlayer: 0, Card: 51}
	b := orig.Marshal()
	if len(b) != SpadesPlaySize {
		t.Fatalf("Play size = %d, want %d", len(b), SpadesPlaySize)
	}
	var got SpadesPlay
	got.Unmarshal(b)
	if got.Seat != 3 || got.NextPlayer != 0 || got.Card != 51 {
		t.Errorf("got seat=%d next=%d card=%d", got.Seat, got.NextPlayer, got.Card)
	}
}

func TestSpadesEndHandMixedEndian(t *testing.T) {
	m := SpadesEndHand{
		Bags: [SpadesNumTeams]int16{3, 7},
		Score: SpadesHandScore{
			BoardNumber: 5,
			Scores:      [SpadesNumTeams]int16{42, -30},
			Base:        [SpadesNumTeams]int16{40, -50},
			BagBonus:    [SpadesNumTeams]int16{2, 0},
			Nil:         [SpadesNumTeams]int16{0, 100},
			BagPenalty:  [SpadesNumTeams]int16{0, -100},
		},
	}
	b := m.Marshal()
	if len(b) != SpadesEndHandSize {
		t.Fatalf("EndHand size = %d, want %d", len(b), SpadesEndHandSize)
	}
	// bags[0] at offset 0: BE
	if int16(wire.ReadBE16(b[0:])) != 3 {
		t.Errorf("bags[0] BE = %d, want 3", int16(wire.ReadBE16(b[0:])))
	}
	// bags[1] at offset 2: BE
	if int16(wire.ReadBE16(b[2:])) != 7 {
		t.Errorf("bags[1] BE = %d, want 7", int16(wire.ReadBE16(b[2:])))
	}
	// boardNumber at offset 4: LE
	if int16(wire.ReadLE16(b[4:])) != 5 {
		t.Errorf("boardNumber LE = %d, want 5", int16(wire.ReadLE16(b[4:])))
	}
	// scores[0] at offset 12: LE
	if int16(wire.ReadLE16(b[12:])) != 42 {
		t.Errorf("scores[0] LE = %d, want 42", int16(wire.ReadLE16(b[12:])))
	}
	// base[0] at offset 20: LE
	if int16(wire.ReadLE16(b[20:])) != 40 {
		t.Errorf("base[0] LE = %d, want 40", int16(wire.ReadLE16(b[20:])))
	}
	// nil[1] at offset 30: LE
	if int16(wire.ReadLE16(b[30:])) != 100 {
		t.Errorf("nil[1] LE = %d, want 100", int16(wire.ReadLE16(b[30:])))
	}
	// bagpenalty[1] at offset 34: LE
	if int16(wire.ReadLE16(b[34:])) != -100 {
		t.Errorf("bagpenalty[1] LE = %d, want -100", int16(wire.ReadLE16(b[34:])))
	}
}

func TestSpadesEndGameSize(t *testing.T) {
	m := SpadesEndGame{Winners: [SpadesNumPlayers]byte{1, 0, 1, 0}}
	b := m.Marshal()
	if len(b) != SpadesEndGameSize {
		t.Fatalf("EndGame size = %d, want %d", len(b), SpadesEndGameSize)
	}
	if b[0] != 1 || b[1] != 0 || b[2] != 1 || b[3] != 0 {
		t.Errorf("winners = %v", b[:4])
	}
}

func TestSpadesNewGameSize(t *testing.T) {
	b := make([]byte, SpadesNewGameSize)
	wire.WriteBE16(b[0:], 2)
	var m SpadesNewGame
	m.Unmarshal(b)
	if m.Seat != 2 {
		t.Errorf("seat = %d, want 2", m.Seat)
	}
}
