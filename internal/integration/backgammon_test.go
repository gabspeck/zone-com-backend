package integration

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"zone.com/internal/backgammon"
	"zone.com/internal/proto"
	"zone.com/internal/server"
	"zone.com/internal/wire"
)

// ---------------------------------------------------------------------------
// testClient — simulates a Windows XP Backgammon client at the socket level
// ---------------------------------------------------------------------------

type testClient struct {
	t       *testing.T
	conn    net.Conn
	reader  *wire.FrameReader
	writer  *wire.FrameWriter
	key     uint32
	channel uint32
	userID  uint32
	seat    int16
	gameID  uint32
}

// connect dials the server.
func (c *testClient) connect(addr string) {
	c.t.Helper()
	var err error
	for i := 0; i < 40; i++ {
		c.conn, err = net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			c.conn.SetDeadline(time.Now().Add(10 * time.Second))
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.t.Fatalf("connect: %v", err)
}

// handshake performs the client-side connection handshake.
func (c *testClient) handshake() {
	c.t.Helper()
	c.key = 0x42424242

	// Build HiMsg (48 bytes)
	hi := make([]byte, wire.HiMsgSize)
	wire.WriteLE32(hi[0:], wire.ConnSig)
	wire.WriteLE32(hi[4:], wire.HiMsgSize)
	wire.WriteLE16(hi[8:], wire.MsgTypeHi)
	wire.WriteLE16(hi[10:], wire.HiMsgSize)
	wire.WriteLE32(hi[12:], wire.ConnVersion)
	wire.WriteLE32(hi[16:], proto.ProductSigFree)
	wire.WriteLE32(hi[20:], wire.OptionAggGeneric|wire.OptionClientKey)
	wire.WriteLE32(hi[24:], wire.OptionAggGeneric|wire.OptionClientKey)
	wire.WriteLE32(hi[28:], c.key)
	// bytes 32-47: MachineGUID — leave as zeros

	wire.Encrypt(hi, wire.DefaultKey)
	if _, err := c.conn.Write(hi); err != nil {
		c.t.Fatalf("handshake write hi: %v", err)
	}

	// Read HelloMsg (40 bytes)
	hello := make([]byte, wire.HelloMsgSize)
	if _, err := io.ReadFull(c.conn, hello); err != nil {
		c.t.Fatalf("handshake read hello: %v", err)
	}
	wire.Decrypt(hello, wire.DefaultKey)

	sig := wire.ReadLE32(hello[0:])
	if sig != wire.ConnSig {
		c.t.Fatalf("hello bad sig: %08x", sig)
	}
	sessionKey := wire.ReadLE32(hello[16:])
	firstSeq := wire.ReadLE32(hello[12:])

	c.key = sessionKey
	c.reader = wire.NewFrameReader(c.conn, c.key, firstSeq)
	c.writer = wire.NewFrameWriter(c.conn, c.key, firstSeq)
}

// proxyNegotiate performs proxy negotiation for the given service.
func (c *testClient) proxyNegotiate(service string) {
	c.t.Helper()
	c.channel = 7

	// Build packed proxy sub-messages: ProxyHi + MillID + ServiceRequest
	proxyHi := make([]byte, proto.ProxyHiMsgSize)
	wire.WriteLE16(proxyHi[0:], uint16(proto.ProxyMsgHi))
	wire.WriteLE16(proxyHi[2:], proto.ProxyHiMsgSize)
	wire.WriteLE32(proxyHi[4:], proto.ProxyVersion)
	copy(proxyHi[8:], "BCKGZM")
	wire.WriteLE32(proxyHi[72:], 17311569)

	millID := make([]byte, proto.ProxyMillIDMsgSize)
	wire.WriteLE16(millID[0:], uint16(proto.ProxyMsgMillID))
	wire.WriteLE16(millID[2:], proto.ProxyMillIDMsgSize)
	wire.WriteLE16(millID[4:], 0x0409) // sysLang
	wire.WriteLE16(millID[6:], 0x0409) // userLang
	wire.WriteLE16(millID[8:], 0x0409) // appLang

	svcReq := make([]byte, proto.ProxyServiceReqSize)
	wire.WriteLE16(svcReq[0:], uint16(proto.ProxyMsgServiceRequest))
	wire.WriteLE16(svcReq[2:], proto.ProxyServiceReqSize)
	wire.WriteLE32(svcReq[4:], proto.ProxyRequestConnect)
	copy(svcReq[8:], service)
	wire.WriteLE32(svcReq[24:], c.channel)

	data := make([]byte, 0, len(proxyHi)+len(millID)+len(svcReq))
	data = append(data, proxyHi...)
	data = append(data, millID...)
	data = append(data, svcReq...)

	err := c.writer.WriteGeneric([]wire.AppMessage{{
		Signature: proto.ProxySig,
		Channel:   0,
		Type:      3,
		Data:      data,
	}})
	if err != nil {
		c.t.Fatalf("proxy write: %v", err)
	}

	// Read proxy responses — server sends 2 frames of proxy sub-messages
	for i := 0; i < 2; i++ {
		c.readFrame() // consume, we trust the server
	}
}

// roomBootstrap sends ClientConfig. Does NOT read responses — that is done
// separately via readBootstrapResponses so both clients can be connected first.
func (c *testClient) sendClientConfig() {
	c.t.Helper()
	cfg := make([]byte, proto.RoomClientConfigSize)
	wire.WriteLE32(cfg[0:], proto.LobbySig)
	wire.WriteLE32(cfg[4:], proto.GameRoomVersion)
	copy(cfg[8:], "Skill=<Beginner>Chat=<On>ILCID=<1033>")

	err := c.writer.WriteGeneric([]wire.AppMessage{{
		Signature: proto.LobbySig,
		Channel:   c.channel,
		Type:      proto.RoomMsgClientConfig,
		Data:      cfg,
	}})
	if err != nil {
		c.t.Fatalf("clientConfig write: %v", err)
	}
}

// readBootstrapResponses reads until it has ZUserIDResponse and StartGameM.
func (c *testClient) readBootstrapResponses() {
	c.t.Helper()
	gotUserID := false
	gotStartGame := false

	deadline := time.Now().Add(8 * time.Second)
	c.conn.SetDeadline(deadline)

	for !gotUserID || !gotStartGame {
		msgs := c.readFrame()
		for _, m := range msgs {
			if m.Signature != proto.LobbySig {
				continue
			}
			switch m.Type {
			case proto.RoomMsgZUserIDResponse:
				if len(m.Data) >= 4 {
					c.userID = wire.ReadLE32(m.Data[0:])
					gotUserID = true
				}
			case proto.RoomMsgStartGameM:
				if len(m.Data) >= 12 {
					c.gameID = wire.ReadLE32(m.Data[0:])
					c.seat = int16(wire.ReadLE16(m.Data[6:]))
					gotStartGame = true
				}
			}
		}
	}
	c.conn.SetDeadline(time.Now().Add(10 * time.Second))
}

// sendGameMsg sends a game message wrapped in RoomGameMessage.
func (c *testClient) sendGameMsg(msgType uint32, payload []byte) {
	c.t.Helper()
	wrapped := proto.MarshalRoomGameMessage(c.gameID, msgType, payload)
	err := c.writer.WriteGeneric([]wire.AppMessage{{
		Signature: proto.LobbySig,
		Channel:   c.channel,
		Type:      proto.RoomMsgGameMessage,
		Data:      wrapped,
	}})
	if err != nil {
		c.t.Fatalf("sendGameMsg: %v", err)
	}
}

// readGameMsg reads the next game message, skipping non-game messages.
func (c *testClient) readGameMsg() (msgType uint32, payload []byte) {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	c.conn.SetDeadline(deadline)
	defer c.conn.SetDeadline(time.Now().Add(10 * time.Second))

	for {
		msgs := c.readFrame()
		for _, m := range msgs {
			if m.Signature == proto.LobbySig && m.Type == proto.RoomMsgGameMessage && len(m.Data) >= proto.RoomGameMessageHdr {
				var gm proto.RoomGameMessage
				gm.Unmarshal(m.Data)
				return gm.MessageType, m.Data[proto.RoomGameMessageHdr:]
			}
		}
	}
}

// readGameMsgOfType reads game messages until the expected type arrives.
func (c *testClient) readGameMsgOfType(expected uint32) []byte {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	c.conn.SetDeadline(deadline)
	defer c.conn.SetDeadline(time.Now().Add(10 * time.Second))

	for {
		msgs := c.readFrame()
		for _, m := range msgs {
			if m.Signature == proto.LobbySig && m.Type == proto.RoomMsgGameMessage && len(m.Data) >= proto.RoomGameMessageHdr {
				var gm proto.RoomGameMessage
				gm.Unmarshal(m.Data)
				if gm.MessageType == expected {
					return m.Data[proto.RoomGameMessageHdr:]
				}
			}
		}
	}
}

// readFrame reads the next Generic frame, handling keepalives and pings.
func (c *testClient) readFrame() []wire.AppMessage {
	c.t.Helper()
	for {
		msgs, ping, err := c.reader.ReadNextFrame()
		if err != nil {
			c.t.Fatalf("readFrame: %v", err)
		}
		if ping != nil {
			// Respond to connection-layer ping
			c.writer.WritePingResponse(ping.YourClk)
			continue
		}
		if msgs == nil {
			// Keepalive — read again
			continue
		}
		// Handle internal app-layer pings
		filtered := make([]wire.AppMessage, 0, len(msgs))
		for _, m := range msgs {
			if m.Signature == proto.InternalAppSig {
				if m.Type == proto.ConnectionPing {
					c.writer.WriteGeneric([]wire.AppMessage{{
						Signature: proto.InternalAppSig,
						Channel:   0,
						Type:      proto.ConnectionPingReply,
						Data:      []byte{0, 0, 0, 0},
					}})
				}
				continue
			}
			filtered = append(filtered, m)
		}
		if len(filtered) > 0 {
			return filtered
		}
	}
}

func (c *testClient) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func startServer(t *testing.T) (string, context.CancelFunc) {
	t.Helper()

	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	srv := server.New(port, 4)

	go func() {
		if err := srv.Run(ctx); err != nil {
			// Server returns nil on clean shutdown
			t.Logf("server exited: %v", err)
		}
	}()

	t.Cleanup(cancel)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return addr, cancel
}

// connectFourClients connects 4 clients through the full bootstrap for a given service.
func connectFourClients(t *testing.T, addr, service string) [4]*testClient {
	t.Helper()

	var clients [4]*testClient
	for i := range clients {
		clients[i] = &testClient{t: t}
		t.Cleanup(clients[i].close)
	}

	for i := range clients {
		clients[i].connect(addr)
		clients[i].handshake()
		clients[i].proxyNegotiate(service)
		clients[i].sendClientConfig()
	}

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

// clientBySeat returns the client sitting at the given seat.
func clientBySeat(clients [4]*testClient, seat int16) *testClient {
	for _, c := range clients {
		if c.seat == seat {
			return c
		}
	}
	return nil
}

func connectTwoClients(t *testing.T, addr string) (*testClient, *testClient) {
	t.Helper()

	c1 := &testClient{t: t}
	c2 := &testClient{t: t}
	t.Cleanup(c1.close)
	t.Cleanup(c2.close)

	// Connect and bootstrap both clients (send phase only)
	c1.connect(addr)
	c1.handshake()
	c1.proxyNegotiate("mbckg_zm_***")
	c1.sendClientConfig()

	c2.connect(addr)
	c2.handshake()
	c2.proxyNegotiate("mbckg_zm_***")
	c2.sendClientConfig()

	// Read bootstrap responses in parallel (StartGameM only comes after both connected)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c1.readBootstrapResponses()
	}()
	go func() {
		defer wg.Done()
		c2.readBootstrapResponses()
	}()
	wg.Wait()

	if c1.userID == 0 || c2.userID == 0 {
		t.Fatalf("bootstrap failed: c1.userID=%d c2.userID=%d", c1.userID, c2.userID)
	}
	if c1.gameID != c2.gameID {
		t.Fatalf("game ID mismatch: c1=%d c2=%d", c1.gameID, c2.gameID)
	}
	if c1.seat == c2.seat {
		t.Fatalf("same seat: both %d", c1.seat)
	}

	return c1, c2
}

// marshalCheckIn builds a 20-byte CheckIn payload with BE fields.
func marshalCheckIn(playerID uint32, seat int16) []byte {
	ci := proto.GameCheckIn{
		ProtocolSignature: backgammon.ProtocolSignature,
		ProtocolVersion:   backgammon.ProtocolVersion,
		ClientVersion:     17311569,
		PlayerID:          playerID,
		Seat:              seat,
		PlayerType:        0,
	}
	return ci.Marshal()
}

// marshalTransaction builds a transaction payload with LE fields.
func marshalTransaction(userID uint32, seat int32, transTag int32, items []proto.BackgammonTransactionItem) []byte {
	tx := proto.BackgammonTransaction{
		User:     userID,
		Seat:     seat,
		TransCnt: int32(len(items)),
		TransTag: transTag,
		Items:    items,
	}
	return tx.Marshal()
}

// dumpEntryOffset returns the byte offset within a SharedState dump for the
// given tag and index.
func dumpEntryOffset(tag, idx int) int {
	off := 0
	for i := 0; i < tag; i++ {
		off += backgammon.SharedStateCounts[i] * 4
	}
	return off + idx*4
}

// readDumpInt32 reads an LE int32 from a SharedState dump.
func readDumpInt32(dump []byte, tag, idx int) int32 {
	off := dumpEntryOffset(tag, idx)
	return int32(binary.LittleEndian.Uint32(dump[off:]))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBackgammonFullSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	// Determine HOST (seat 0) and GUEST (seat 1)
	host, guest := c1, c2
	if c1.seat != 0 {
		host, guest = c2, c1
	}

	// Both send CheckIn
	host.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(host.userID, host.seat))
	guest.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(guest.userID, guest.seat))

	// Each receives 2 CheckIns (self-echo + opponent)
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)

	// HOST sends InitSettings transaction
	initSettingsItems := []proto.BackgammonTransactionItem{
		{EntryTag: backgammon.StateTagHostBrown, EntryIdx: -1, EntryVal: 1},
		{EntryTag: backgammon.StateTagTargetScore, EntryIdx: -1, EntryVal: 3},
		{EntryTag: backgammon.StateTagAutoDouble, EntryIdx: -1, EntryVal: 0},
	}
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransInitSettings, initSettingsItems))

	// Guest receives the transaction
	payload := guest.readGameMsgOfType(proto.BackgammonMsgTransaction)
	if len(payload) < proto.BackgammonTransactionHdrSize {
		t.Fatalf("transaction too short: %d", len(payload))
	}
	var rxTx proto.BackgammonTransaction
	rxTx.Unmarshal(payload)
	if rxTx.TransTag != backgammon.TransInitSettings {
		t.Fatalf("expected TransInitSettings(%d), got %d", backgammon.TransInitSettings, rxTx.TransTag)
	}

	// HOST drives state to InitialRoll
	stateChangeItems := []proto.BackgammonTransactionItem{
		{EntryTag: backgammon.StateTagBGState, EntryIdx: -1, EntryVal: backgammon.StateInitialRoll},
	}
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransStateChange, stateChangeItems))
	guest.readGameMsgOfType(proto.BackgammonMsgTransaction) // consume relay

	// HOST sends RollRequest
	rollReq := make([]byte, 2)
	wire.WriteBE16(rollReq, uint16(host.seat))
	host.sendGameMsg(proto.BackgammonMsgRollRequest, rollReq)

	// Both receive DiceRoll
	hostDice := host.readGameMsgOfType(proto.BackgammonMsgDiceRoll)
	guestDice := guest.readGameMsgOfType(proto.BackgammonMsgDiceRoll)

	if len(hostDice) == 0 || len(guestDice) == 0 {
		t.Fatal("did not receive DiceRoll")
	}

	t.Logf("Full session completed: host=%d(seat %d) guest=%d(seat %d) gameID=%d diceLen=%d",
		host.userID, host.seat, guest.userID, guest.seat, host.gameID, len(hostDice))
}

func TestBackgammonCheckInExchange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	// Both send CheckIn
	c1.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(c1.userID, c1.seat))
	c2.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(c2.userID, c2.seat))

	// Each client should receive exactly 2 CheckIn messages (self-echo + opponent)
	type checkInResult struct {
		msgs []proto.GameCheckIn
	}

	readCheckIns := func(c *testClient) checkInResult {
		var result checkInResult
		for i := 0; i < 2; i++ {
			payload := c.readGameMsgOfType(proto.GameMsgCheckIn)
			if len(payload) < proto.BackgammonCheckInSize {
				t.Fatalf("CheckIn payload too short: %d", len(payload))
			}
			var ci proto.GameCheckIn
			ci.Unmarshal(payload)
			result.msgs = append(result.msgs, ci)
		}
		return result
	}

	var r1, r2 checkInResult
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r1 = readCheckIns(c1)
	}()
	go func() {
		defer wg.Done()
		r2 = readCheckIns(c2)
	}()
	wg.Wait()

	// Validate c1's received CheckIns
	gotSelf, gotOpp := false, false
	for _, ci := range r1.msgs {
		if ci.ProtocolSignature != backgammon.ProtocolSignature {
			t.Errorf("c1: bad protocol sig %08x", ci.ProtocolSignature)
		}
		if ci.ProtocolVersion != backgammon.ProtocolVersion {
			t.Errorf("c1: bad protocol version %d", ci.ProtocolVersion)
		}
		if ci.Seat == c1.seat && ci.PlayerID == c1.userID {
			gotSelf = true
		}
		if ci.Seat == c2.seat && ci.PlayerID == c2.userID {
			gotOpp = true
		}
	}
	if !gotSelf {
		t.Error("c1 did not receive self-echo CheckIn")
	}
	if !gotOpp {
		t.Error("c1 did not receive opponent CheckIn")
	}

	// Validate c2's received CheckIns
	gotSelf, gotOpp = false, false
	for _, ci := range r2.msgs {
		if ci.Seat == c2.seat && ci.PlayerID == c2.userID {
			gotSelf = true
		}
		if ci.Seat == c1.seat && ci.PlayerID == c1.userID {
			gotOpp = true
		}
	}
	if !gotSelf {
		t.Error("c2 did not receive self-echo CheckIn")
	}
	if !gotOpp {
		t.Error("c2 did not receive opponent CheckIn")
	}
}

func TestBackgammonDiceRollLayout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	host, guest := c1, c2
	if c1.seat != 0 {
		host, guest = c2, c1
	}

	// CheckIn exchange (self-echo + opponent)
	host.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(host.userID, host.seat))
	guest.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(guest.userID, guest.seat))
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)

	// Drive state to InitialRoll via transactions
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransStateChange,
			[]proto.BackgammonTransactionItem{
				{EntryTag: backgammon.StateTagBGState, EntryIdx: -1, EntryVal: backgammon.StateInitialRoll},
			}))
	guest.readGameMsgOfType(proto.BackgammonMsgTransaction)

	// Send RollRequest
	rollReq := make([]byte, 2)
	wire.WriteBE16(rollReq, uint16(host.seat))
	host.sendGameMsg(proto.BackgammonMsgRollRequest, rollReq)

	// Read DiceRoll
	payload := host.readGameMsgOfType(proto.BackgammonMsgDiceRoll)
	guest.readGameMsgOfType(proto.BackgammonMsgDiceRoll) // consume

	// MSVC sizeof(ZBGMsgDiceRoll) = 36 bytes:
	//   seat(2) + pad(2) + DICEINFO(16) + DICEINFO(16) = 36
	// where DICEINFO = Value(2) + pad(2) + EncodedValue(4) + EncoderMul(2) + EncoderAdd(2) + numUses(4) = 16
	const expectedDiceRollSize = 36
	if len(payload) != expectedDiceRollSize {
		t.Fatalf("DiceRoll size mismatch: server sent %d bytes, XP client expects %d bytes "+
			"(DICEINFO struct has 2 bytes padding after int16 Value under MSVC default alignment, "+
			"and ZBGMsgDiceRoll has 2 bytes padding after ZSeat seat)",
			len(payload), expectedDiceRollSize)
	}

	// Validate MSVC-aligned field offsets
	seat := int16(wire.ReadBE16(payload[0:]))
	if seat != host.seat {
		t.Errorf("DiceRoll seat: got %d, want %d", seat, host.seat)
	}

	// d1 at offset 4 (after 2-byte padding)
	d1Value := int16(wire.ReadBE16(payload[4:]))
	if d1Value < 1 || d1Value > 6 {
		t.Errorf("d1.Value out of range: %d", d1Value)
	}

	d1Encoded := int32(wire.ReadBE32(payload[8:]))
	d1Mul := int16(wire.ReadBE16(payload[12:]))
	d1Add := int16(wire.ReadBE16(payload[14:]))
	expectedEncoded := (int32(d1Value)*int32(d1Mul) + int32(d1Add)) * 384 + 47
	if d1Encoded != expectedEncoded {
		t.Errorf("d1.EncodedValue: got %d, want %d", d1Encoded, expectedEncoded)
	}

	// d2 at offset 20
	d2Value := int16(wire.ReadBE16(payload[20:]))
	if d2Value < 1 || d2Value > 6 {
		t.Errorf("d2.Value out of range: %d", d2Value)
	}
}

func TestBackgammonTransactionRelay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	host, guest := c1, c2
	if c1.seat != 0 {
		host, guest = c2, c1
	}

	// CheckIn (self-echo + opponent)
	host.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(host.userID, host.seat))
	guest.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(guest.userID, guest.seat))
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)

	// HOST sends InitSettings
	sentItems := []proto.BackgammonTransactionItem{
		{EntryTag: backgammon.StateTagHostBrown, EntryIdx: -1, EntryVal: 1},
		{EntryTag: backgammon.StateTagTargetScore, EntryIdx: -1, EntryVal: 5},
		{EntryTag: backgammon.StateTagAutoDouble, EntryIdx: -1, EntryVal: 1},
		{EntryTag: backgammon.StateTagAllowWatching, EntryIdx: 0, EntryVal: 0},
	}
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransInitSettings, sentItems))

	// Guest receives it
	payload := guest.readGameMsgOfType(proto.BackgammonMsgTransaction)
	var rx proto.BackgammonTransaction
	rx.Unmarshal(payload)

	if rx.User != host.userID {
		t.Errorf("tx User: got %d, want %d", rx.User, host.userID)
	}
	if rx.Seat != int32(host.seat) {
		t.Errorf("tx Seat: got %d, want %d", rx.Seat, host.seat)
	}
	if rx.TransTag != backgammon.TransInitSettings {
		t.Errorf("tx TransTag: got %d, want %d", rx.TransTag, backgammon.TransInitSettings)
	}
	if int(rx.TransCnt) != len(sentItems) {
		t.Fatalf("tx TransCnt: got %d, want %d", rx.TransCnt, len(sentItems))
	}
	for i, item := range rx.Items {
		if item.EntryTag != sentItems[i].EntryTag {
			t.Errorf("item[%d].EntryTag: got %d, want %d", i, item.EntryTag, sentItems[i].EntryTag)
		}
		if item.EntryIdx != sentItems[i].EntryIdx {
			t.Errorf("item[%d].EntryIdx: got %d, want %d", i, item.EntryIdx, sentItems[i].EntryIdx)
		}
		if item.EntryVal != sentItems[i].EntryVal {
			t.Errorf("item[%d].EntryVal: got %d, want %d", i, item.EntryVal, sentItems[i].EntryVal)
		}
	}

	// HOST sends StateChange
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransStateChange,
			[]proto.BackgammonTransactionItem{
				{EntryTag: backgammon.StateTagBGState, EntryIdx: -1, EntryVal: backgammon.StateInitialRoll},
				{EntryTag: backgammon.StateTagActiveSeat, EntryIdx: -1, EntryVal: 0},
			}))

	payload = guest.readGameMsgOfType(proto.BackgammonMsgTransaction)
	rx = proto.BackgammonTransaction{}
	rx.Unmarshal(payload)
	if rx.TransTag != backgammon.TransStateChange {
		t.Errorf("StateChange tx TransTag: got %d, want %d", rx.TransTag, backgammon.TransStateChange)
	}
	if len(rx.Items) != 2 {
		t.Fatalf("StateChange tx items: got %d, want 2", len(rx.Items))
	}
	if rx.Items[0].EntryVal != backgammon.StateInitialRoll {
		t.Errorf("StateChange BGState: got %d, want %d", rx.Items[0].EntryVal, backgammon.StateInitialRoll)
	}
}

func TestBackgammonGameStateRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	// CheckIn both (self-echo + opponent)
	c1.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(c1.userID, c1.seat))
	c2.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(c2.userID, c2.seat))
	c1.readGameMsgOfType(proto.GameMsgCheckIn)
	c1.readGameMsgOfType(proto.GameMsgCheckIn)
	c2.readGameMsgOfType(proto.GameMsgCheckIn)
	c2.readGameMsgOfType(proto.GameMsgCheckIn)

	// c1 sends GameStateRequest
	gsReq := make([]byte, proto.BackgammonGameStateReqSize)
	wire.WriteBE32(gsReq[0:], c1.userID)
	wire.WriteBE16(gsReq[4:], uint16(c1.seat))
	wire.WriteLE16(gsReq[6:], 0)
	c1.sendGameMsg(proto.GameMsgGameStateRequest, gsReq)

	// Read GameStateResponse
	payload := c1.readGameMsgOfType(proto.GameMsgGameStateResponse)

	// Response format: playerID(4,BE) + seat(2,BE) + rfu(2,LE) + dumpLen(4,LE) + dump(N) + rfu(4,LE)
	if len(payload) < 12 {
		t.Fatalf("GameStateResponse too short: %d bytes", len(payload))
	}

	respPlayerID := wire.ReadBE32(payload[0:])
	if respPlayerID != c1.userID {
		t.Errorf("GameStateResponse playerID: got %d, want %d", respPlayerID, c1.userID)
	}

	dumpLen := int(wire.ReadLE32(payload[8:]))
	// SharedState has 65 int32 entries (sum of SharedStateCounts) = 260 bytes
	expectedDumpLen := 0
	for _, cnt := range backgammon.SharedStateCounts {
		expectedDumpLen += cnt * 4
	}
	if dumpLen != expectedDumpLen {
		t.Fatalf("dump length: got %d, want %d", dumpLen, expectedDumpLen)
	}

	expectedTotalLen := 8 + 4 + dumpLen + 4
	if len(payload) < expectedTotalLen {
		t.Fatalf("payload too short for dump: got %d, want >= %d", len(payload), expectedTotalLen)
	}

	dump := payload[12 : 12+dumpLen]

	// Verify initial SharedState values
	bgState := readDumpInt32(dump, backgammon.StateTagBGState, 0)
	if bgState != backgammon.StateNotInit {
		t.Errorf("BGState: got %d, want %d (NotInit)", bgState, backgammon.StateNotInit)
	}

	activeSeat := readDumpInt32(dump, backgammon.StateTagActiveSeat, 0)
	if activeSeat != 0 {
		t.Errorf("ActiveSeat: got %d, want 0", activeSeat)
	}

	targetScore := readDumpInt32(dump, backgammon.StateTagTargetScore, 0)
	if targetScore != 3 {
		t.Errorf("TargetScore: got %d, want 3", targetScore)
	}

	cubeValue := readDumpInt32(dump, backgammon.StateTagCubeValue, 0)
	if cubeValue != 1 {
		t.Errorf("CubeValue: got %d, want 1", cubeValue)
	}

	// Verify initial piece positions
	for i, expected := range backgammon.InitPiecePositions {
		got := readDumpInt32(dump, backgammon.StateTagPieces, i)
		if got != expected {
			t.Errorf("Piece[%d]: got %d, want %d", i, got, expected)
		}
	}

	// Verify UserIDs were set during CheckIn
	uid0 := readDumpInt32(dump, backgammon.StateTagUserIDs, 0)
	uid1 := readDumpInt32(dump, backgammon.StateTagUserIDs, 1)
	if uid0 == -1 && uid1 == -1 {
		t.Error("UserIDs not set in state dump")
	}
}

func TestBackgammonDiceEncoding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	addr, _ := startServer(t)
	c1, c2 := connectTwoClients(t, addr)

	host, guest := c1, c2
	if c1.seat != 0 {
		host, guest = c2, c1
	}

	// CheckIn (self-echo + opponent)
	host.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(host.userID, host.seat))
	guest.sendGameMsg(proto.GameMsgCheckIn, marshalCheckIn(guest.userID, guest.seat))
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	host.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)
	guest.readGameMsgOfType(proto.GameMsgCheckIn)

	// Drive to InitialRoll
	host.sendGameMsg(proto.BackgammonMsgTransaction,
		marshalTransaction(host.userID, int32(host.seat), backgammon.TransStateChange,
			[]proto.BackgammonTransactionItem{
				{EntryTag: backgammon.StateTagBGState, EntryIdx: -1, EntryVal: backgammon.StateInitialRoll},
			}))
	guest.readGameMsgOfType(proto.BackgammonMsgTransaction)

	// RollRequest
	rollReq := make([]byte, 2)
	wire.WriteBE16(rollReq, uint16(host.seat))
	host.sendGameMsg(proto.BackgammonMsgRollRequest, rollReq)

	payload := host.readGameMsgOfType(proto.BackgammonMsgDiceRoll)
	guest.readGameMsgOfType(proto.BackgammonMsgDiceRoll)

	// Parse using the MSVC-aligned layout (16-byte DICEINFO with padding).
	if len(payload) < 36 {
		t.Fatalf("DiceRoll too short: %d (want 36)", len(payload))
	}

	// MSVC layout: seat(2) + pad(2) + DICEINFO(16) + DICEINFO(16) = 36
	// DICEINFO: Value(2) + pad(2) + EncodedValue(4) + EncoderMul(2) + EncoderAdd(2) + NumUses(4)
	parseDiceMSVC := func(b []byte) (value, mul, add int16, encoded int32, uses int32) {
		value = int16(wire.ReadBE16(b[0:]))
		// bytes 2-3: padding
		encoded = int32(wire.ReadBE32(b[4:]))
		mul = int16(wire.ReadBE16(b[8:]))
		add = int16(wire.ReadBE16(b[10:]))
		uses = int32(wire.ReadBE32(b[12:]))
		return
	}

	d1Val, d1Mul, d1Add, d1Enc, d1Uses := parseDiceMSVC(payload[4:])  // d1 at offset 4
	d2Val, d2Mul, d2Add, d2Enc, d2Uses := parseDiceMSVC(payload[20:]) // d2 at offset 20

	// Validate Value range
	for _, v := range []int16{d1Val, d2Val} {
		if v < 1 || v > 6 {
			t.Errorf("dice value out of range: %d", v)
		}
	}

	// Validate EncodedValue = (Value * EncoderMul + EncoderAdd) * 384 + 47
	checkEncoded := func(name string, val, mul, add int16, enc int32) {
		expected := (int32(val)*int32(mul) + int32(add)) * 384 + 47
		if enc != expected {
			t.Errorf("%s EncodedValue: got %d, want %d (val=%d mul=%d add=%d)",
				name, enc, expected, val, mul, add)
		}
	}
	checkEncoded("d1", d1Val, d1Mul, d1Add, d1Enc)
	checkEncoded("d2", d2Val, d2Mul, d2Add, d2Enc)

	// Validate Uses encoding: numUses = ((uses*16+31)*(EncoderMul+3)) + (EncoderAdd+4)
	checkUses := func(name string, val int16, mul, add int16, encodedUses int32) {
		var expectedRawUses int32
		if d1Val == d2Val {
			expectedRawUses = 2 // doubles
		} else {
			expectedRawUses = 1
		}
		expected := ((expectedRawUses*16 + 31) * int32(mul+3)) + int32(add+4)
		if encodedUses != expected {
			t.Errorf("%s Uses: got %d, want %d (rawUses=%d mul=%d add=%d)",
				name, encodedUses, expected, expectedRawUses, mul, add)
		}
	}
	checkUses("d1", d1Val, d1Mul, d1Add, d1Uses)
	checkUses("d2", d2Val, d2Mul, d2Add, d2Uses)

	t.Logf("Dice: d1=%d(enc=%d,mul=%d,add=%d,uses=%d) d2=%d(enc=%d,mul=%d,add=%d,uses=%d)",
		d1Val, d1Enc, d1Mul, d1Add, d1Uses,
		d2Val, d2Enc, d2Mul, d2Add, d2Uses)
}
