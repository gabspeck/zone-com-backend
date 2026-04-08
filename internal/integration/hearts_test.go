package integration

import (
	"sync"
	"testing"

	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

// connectFourHeartsClients connects 4 clients through the full bootstrap
// and reads their StartGameM. Returns clients with seat/gameID populated.
func connectFourHeartsClients(t *testing.T, addr string) [4]*testClient {
	t.Helper()

	var clients [4]*testClient
	for i := range clients {
		clients[i] = &testClient{t: t}
		t.Cleanup(clients[i].close)
	}

	// Connect each client sequentially (handshake + proxy + config)
	for i := range clients {
		clients[i].connect(addr)
		clients[i].handshake()
		clients[i].proxyNegotiate("mhrtz_zm_***")
		clients[i].sendClientConfig()
	}

	// Read bootstrap responses in parallel (StartGameM only after all 4 seated)
	var wg sync.WaitGroup
	wg.Add(4)
	for i := range clients {
		i := i
		go func() {
			defer wg.Done()
			clients[i].readBootstrapResponses()
		}()
	}
	wg.Wait()

	// Verify all got unique seats
	seatsSeen := make(map[int16]bool)
	for i, c := range clients {
		t.Logf("client %d: userID=%d seat=%d gameID=%d", i, c.userID, c.seat, c.gameID)
		if seatsSeen[c.seat] {
			t.Fatalf("duplicate seat %d", c.seat)
		}
		seatsSeen[c.seat] = true
	}
	return clients
}

// sendHeartsClientReady sends the ClientReady message from a client.
func sendHeartsClientReady(t *testing.T, c *testClient) {
	t.Helper()
	b := make([]byte, proto.HeartsClientReadySize)
	wire.WriteBE32(b[0:], proto.HeartsSig)
	wire.WriteBE32(b[4:], proto.HeartsVersion)
	wire.WriteBE32(b[8:], 1) // version
	wire.WriteBE16(b[12:], uint16(c.seat))
	c.sendGameMsg(proto.HeartsMsgClientReady, b)
}

func TestHeartsFullSession(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourHeartsClients(t, addr)

	// All 4 send ClientReady
	for i := range clients {
		sendHeartsClientReady(t, clients[i])
	}

	// All 4 should receive StartGame (0x100)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartGame)
		if len(payload) != proto.HeartsStartGameSize {
			t.Fatalf("client %d: StartGame size = %d, want %d", i, len(payload), proto.HeartsStartGameSize)
		}
		numCards := wire.ReadBE16(payload[0:])
		numPass := wire.ReadBE16(payload[2:])
		numPts := wire.ReadBE16(payload[4:])
		t.Logf("client %d: StartGame numCards=%d numPass=%d numPts=%d", i, numCards, numPass, numPts)
		if numCards != 13 || numPass != 3 || numPts != 100 {
			t.Fatalf("client %d: unexpected StartGame params", i)
		}
	}

	// All 4 should receive StartHand (0x102)
	passDir := int16(0)
	hands := [4][proto.HeartsMaxNumCardsInHand]byte{}
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartHand)
		if len(payload) != proto.HeartsStartHandSize {
			t.Fatalf("client %d: StartHand size = %d, want %d", i, len(payload), proto.HeartsStartHandSize)
		}
		passDir = int16(wire.ReadBE16(payload[0:]))
		copy(hands[i][:], payload[2:])
		cardCount := 0
		for _, c := range hands[i] {
			if c != proto.HeartsCardNone {
				cardCount++
			}
		}
		t.Logf("client %d: StartHand passDir=%d cards=%d", i, passDir, cardCount)
		if cardCount != 13 {
			t.Fatalf("client %d: got %d cards, want 13", i, cardCount)
		}
	}

	// If there's a pass phase, do it
	if passDir != int16(proto.HeartsPassHold) {
		// Each client passes their first 3 cards
		for i := range clients {
			var msg proto.HeartsPassCards
			msg.Seat = clients[i].seat
			msg.Pass[0] = hands[i][0]
			msg.Pass[1] = hands[i][1]
			msg.Pass[2] = hands[i][2]
			clients[i].sendGameMsg(proto.HeartsMsgPassCards, msg.Marshal())
		}

		// Each client should receive 4 PassCards broadcasts (one from each seat)
		for i := range clients {
			for j := 0; j < 4; j++ {
				clients[i].readGameMsgOfType(proto.HeartsMsgPassCards)
			}
			t.Logf("client %d: received all 4 PassCards", i)
		}
	}

	// All should receive StartPlay (0x103)
	startSeat := int16(-1)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartPlay)
		if len(payload) != proto.HeartsStartPlaySize {
			t.Fatalf("client %d: StartPlay size = %d, want %d", i, len(payload), proto.HeartsStartPlaySize)
		}
		startSeat = int16(wire.ReadBE16(payload[0:]))
		t.Logf("client %d: StartPlay seat=%d", i, startSeat)
	}

	if startSeat < 0 || startSeat >= 4 {
		t.Fatalf("invalid start seat: %d", startSeat)
	}

	t.Logf("Hearts session bootstrap complete: passDir=%d startSeat=%d", passDir, startSeat)
}

func TestHeartsCheckIn(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourHeartsClients(t, addr)

	// All 4 send CheckIn
	for i := range clients {
		var msg proto.HeartsCheckIn
		msg.UserID = clients[i].userID
		msg.Seat = clients[i].seat
		clients[i].sendGameMsg(proto.HeartsMsgCheckIn, msg.Marshal())
	}

	// Then send ClientReady
	for i := range clients {
		sendHeartsClientReady(t, clients[i])
	}

	// Should get StartGame
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartGame)
		if len(payload) != proto.HeartsStartGameSize {
			t.Fatalf("client %d: StartGame size mismatch", i)
		}
	}
	t.Log("Hearts CheckIn + ClientReady flow works")
}

func TestHeartsPlayCard(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourHeartsClients(t, addr)

	// ClientReady
	for i := range clients {
		sendHeartsClientReady(t, clients[i])
	}

	// Read StartGame
	for i := range clients {
		clients[i].readGameMsgOfType(proto.HeartsMsgStartGame)
	}

	// Read StartHand
	hands := [4][proto.HeartsMaxNumCardsInHand]byte{}
	passDir := int16(0)
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartHand)
		passDir = int16(wire.ReadBE16(payload[0:]))
		copy(hands[i][:], payload[2:])
	}

	// Pass phase if needed
	if passDir != int16(proto.HeartsPassHold) {
		for i := range clients {
			var msg proto.HeartsPassCards
			msg.Seat = clients[i].seat
			msg.Pass[0] = hands[i][0]
			msg.Pass[1] = hands[i][1]
			msg.Pass[2] = hands[i][2]
			clients[i].sendGameMsg(proto.HeartsMsgPassCards, msg.Marshal())
		}
		for i := range clients {
			for j := 0; j < 4; j++ {
				clients[i].readGameMsgOfType(proto.HeartsMsgPassCards)
			}
		}
	}

	// Read StartPlay
	var startPlaySeat int16
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgStartPlay)
		startPlaySeat = int16(wire.ReadBE16(payload[0:]))
	}

	// Find the client that has this seat and play a card
	for i := range clients {
		if clients[i].seat == startPlaySeat {
			// The starting player always has 2C (card 0) after the pass phase.
			msg := proto.HeartsPlayCard{Seat: clients[i].seat, Card: 0}
			clients[i].sendGameMsg(proto.HeartsMsgPlayCard, msg.Marshal())
			t.Logf("client %d (seat %d): played 2C", i, clients[i].seat)

			// All 4 should receive the PlayCard broadcast
			for j := range clients {
				payload := clients[j].readGameMsgOfType(proto.HeartsMsgPlayCard)
				if len(payload) != proto.HeartsPlayCardSize {
					t.Fatalf("client %d: PlayCard size = %d, want %d", j, len(payload), proto.HeartsPlayCardSize)
				}
				gotSeat := int16(wire.ReadBE16(payload[0:]))
				gotCard := payload[2]
				if gotSeat != startPlaySeat || gotCard != cardToPlay {
					t.Fatalf("client %d: PlayCard mismatch seat=%d card=%d", j, gotSeat, gotCard)
				}
			}
			t.Log("PlayCard broadcast verified for all 4 clients")
			return
		}
	}
	t.Fatal("could not find starting player")
}

func TestHeartsTalk(t *testing.T) {
	addr, _ := startServer(t)
	clients := connectFourHeartsClients(t, addr)

	// ClientReady
	for i := range clients {
		sendHeartsClientReady(t, clients[i])
	}
	// Read StartGame + StartHand
	for i := range clients {
		clients[i].readGameMsgOfType(proto.HeartsMsgStartGame)
	}
	for i := range clients {
		clients[i].readGameMsgOfType(proto.HeartsMsgStartHand)
	}

	// Client 0 sends a chat message
	text := []byte("Hello\x00")
	hdr := proto.HeartsTalk{
		UserID:     clients[0].userID,
		Seat:       clients[0].seat,
		MessageLen: uint16(len(text)),
	}
	chatPayload := append(hdr.Marshal(), text...)
	clients[0].sendGameMsg(proto.HeartsMsgTalk, chatPayload)

	// All 4 should receive it
	for i := range clients {
		payload := clients[i].readGameMsgOfType(proto.HeartsMsgTalk)
		if len(payload) < proto.HeartsTalkHdrSize {
			t.Fatalf("client %d: Talk too short", i)
		}
		var got proto.HeartsTalk
		got.Unmarshal(payload)
		if got.UserID != clients[0].userID {
			t.Errorf("client %d: Talk userID = %d, want %d", i, got.UserID, clients[0].userID)
		}
	}
	t.Log("Talk broadcast verified")
}
