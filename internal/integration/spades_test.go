package integration

import (
	"testing"

	"zone.com/internal/proto"
	"zone.com/internal/spades"
	"zone.com/internal/wire"
)

func sendSpadesClientReady(t *testing.T, c *testClient) {
	t.Helper()
	b := make([]byte, proto.SpadesClientReadySize)
	wire.WriteBE32(b[0:], proto.SpadesSig)
	wire.WriteBE32(b[4:], proto.SpadesVersion)
	wire.WriteBE32(b[8:], 1) // version
	wire.WriteBE32(b[12:], c.userID)
	wire.WriteBE16(b[16:], uint16(c.seat))
	c.sendGameMsg(proto.SpadesMsgClientReady, b)
}

func TestSpadesFullSession(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourClients(t, addr, "mshvl_zm_***")

	// All 4 send ClientReady
	for i := range clients {
		sendSpadesClientReady(t, clients[i])
	}

	// All 4 should receive StartGame (0x101)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgStartGame)
		if len(payload) != proto.SpadesStartGameSize {
			t.Fatalf("client %d: StartGame size = %d, want %d", i, len(payload), proto.SpadesStartGameSize)
		}
		numPts := int16(wire.ReadBE16(payload[20:]))
		t.Logf("client %d: StartGame numPointsInGame=%d", i, numPts)
		if numPts != spades.PointsForGame {
			t.Fatalf("client %d: unexpected numPointsInGame %d", i, numPts)
		}
	}

	// All 4 should receive StartBid (0x103)
	hands := [4][proto.SpadesNumCardsInHand]byte{}
	dealer := int16(-1)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgStartBid)
		if len(payload) != proto.SpadesStartBidSize {
			t.Fatalf("client %d: StartBid size = %d, want %d", i, len(payload), proto.SpadesStartBidSize)
		}
		boardNum := int16(wire.ReadBE16(payload[0:]))
		dealer = int16(wire.ReadBE16(payload[2:]))
		copy(hands[i][:], payload[4:])
		cardCount := 0
		for _, c := range hands[i] {
			if c != spades.CardNone && c < 52 {
				cardCount++
			}
		}
		t.Logf("client %d (seat %d): StartBid board=%d dealer=%d cards=%d",
			i, clients[i].seat, boardNum, dealer, cardCount)
		if cardCount != 13 {
			t.Fatalf("client %d: got %d cards, want 13", i, cardCount)
		}
	}

	t.Logf("Spades session bootstrap complete: dealer=%d", dealer)
}

func TestSpadesBidding(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourClients(t, addr, "mshvl_zm_***")

	for i := range clients {
		sendSpadesClientReady(t, clients[i])
	}

	// Read StartGame
	for i := range clients {
		clients[i].readGameMsgOfType(proto.SpadesMsgStartGame)
	}

	// Read StartBid — get dealer
	dealer := int16(0)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgStartBid)
		dealer = int16(wire.ReadBE16(payload[2:]))
	}

	// Bid in order: dealer, dealer+1, dealer+2, dealer+3
	bids := [4]byte{3, 4, 2, 4}
	for k := 0; k < 4; k++ {
		seat := (dealer + int16(k)) % 4
		c := clientBySeat(clients, seat)
		msg := proto.SpadesBid{Seat: seat, Bid: bids[k]}
		c.sendGameMsg(proto.SpadesMsgBid, msg.Marshal())
		t.Logf("seat %d bid %d", seat, bids[k])

		// All 4 should receive the Bid broadcast
		for i := range clients {
			payload := clients[i].readGameMsgOfType(proto.SpadesMsgBid)
			if len(payload) != proto.SpadesBidSize {
				t.Fatalf("client %d: Bid size = %d, want %d", i, len(payload), proto.SpadesBidSize)
			}
			var got proto.SpadesBid
			got.Unmarshal(payload)
			if got.Seat != seat {
				t.Fatalf("client %d: Bid seat = %d, want %d", i, got.Seat, seat)
			}
			if got.Bid != bids[k] {
				t.Fatalf("client %d: Bid value = %d, want %d", i, got.Bid, bids[k])
			}
		}
	}

	// After all 4 bids, should receive StartPlay (0x105)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgStartPlay)
		if len(payload) != proto.SpadesStartPlaySize {
			t.Fatalf("client %d: StartPlay size = %d, want %d", i, len(payload), proto.SpadesStartPlaySize)
		}
		leader := int16(wire.ReadBE16(payload[0:]))
		t.Logf("client %d: StartPlay leader=%d", i, leader)
		if leader != dealer {
			t.Fatalf("client %d: leader = %d, want dealer %d", i, leader, dealer)
		}
	}

	t.Log("Spades bidding phase complete")
}

func TestSpadesPlayCard(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourClients(t, addr, "mshvl_zm_***")

	for i := range clients {
		sendSpadesClientReady(t, clients[i])
	}
	for i := range clients {
		clients[i].readGameMsgOfType(proto.SpadesMsgStartGame)
	}

	// Read StartBid — collect hands by seat
	dealer := int16(0)
	handsBySeat := [4][proto.SpadesNumCardsInHand]byte{}
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgStartBid)
		dealer = int16(wire.ReadBE16(payload[2:]))
		copy(handsBySeat[clients[i].seat][:], payload[4:])
	}

	// All bid 3
	for k := 0; k < 4; k++ {
		seat := (dealer + int16(k)) % 4
		c := clientBySeat(clients, seat)
		msg := proto.SpadesBid{Seat: seat, Bid: 3}
		c.sendGameMsg(proto.SpadesMsgBid, msg.Marshal())
		for i := range clients {
			clients[i].readGameMsgOfType(proto.SpadesMsgBid)
		}
	}

	// Read StartPlay
	for i := range clients {
		clients[i].readGameMsgOfType(proto.SpadesMsgStartPlay)
	}

	// Dealer leads — pick their first non-spade card (trumps not broken)
	leaderSeat := dealer
	leaderClient := clientBySeat(clients, leaderSeat)
	var cardToPlay byte = spades.CardNone
	for _, c := range handsBySeat[leaderSeat] {
		if c != spades.CardNone && c < 52 && spades.Suit(c) != spades.SuitSpades {
			cardToPlay = c
			break
		}
	}
	if cardToPlay == spades.CardNone {
		// All spades — pick any
		cardToPlay = handsBySeat[leaderSeat][0]
	}

	msg := proto.SpadesPlay{Seat: leaderSeat, Card: cardToPlay}
	leaderClient.sendGameMsg(proto.SpadesMsgPlay, msg.Marshal())
	t.Logf("seat %d played card %d (suit %d)", leaderSeat, cardToPlay, spades.Suit(cardToPlay))

	// All 4 should receive the Play broadcast
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgPlay)
		if len(payload) != proto.SpadesPlaySize {
			t.Fatalf("client %d: Play size = %d, want %d", i, len(payload), proto.SpadesPlaySize)
		}
		var got proto.SpadesPlay
		got.Unmarshal(payload)
		if got.Seat != leaderSeat || got.Card != cardToPlay {
			t.Fatalf("client %d: Play mismatch seat=%d card=%d", i, got.Seat, got.Card)
		}
	}
	t.Log("PlayCard broadcast verified for all 4 clients")
}

func TestSpadesTalk(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourClients(t, addr, "mshvl_zm_***")

	for i := range clients {
		sendSpadesClientReady(t, clients[i])
	}
	for i := range clients {
		clients[i].readGameMsgOfType(proto.SpadesMsgStartGame)
	}
	for i := range clients {
		clients[i].readGameMsgOfType(proto.SpadesMsgStartBid)
	}

	// Client 0 sends a chat message
	text := []byte("Hello Spades\x00")
	hdr := proto.SpadesTalk{
		PlayerID:   clients[0].userID,
		MessageLen: uint16(len(text)),
	}
	chatPayload := append(hdr.Marshal(), text...)
	clients[0].sendGameMsg(proto.SpadesMsgTalk, chatPayload)

	// All 4 should receive it
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.SpadesMsgTalk)
		if len(payload) < proto.SpadesTalkHdrSize {
			t.Fatalf("client %d: Talk too short", i)
		}
		var got proto.SpadesTalk
		got.Unmarshal(payload)
		if got.PlayerID != clients[0].userID {
			t.Errorf("client %d: Talk userID = %d, want %d", i, got.PlayerID, clients[0].userID)
		}
	}
	t.Log("Spades Talk broadcast verified")
}
