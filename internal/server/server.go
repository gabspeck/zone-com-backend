package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"zone.com/internal/conn"
	"zone.com/internal/proto"
	"zone.com/internal/proxy"
	"zone.com/internal/room"
	"zone.com/internal/wire"
)

// Server is the Zone.com checkers game server.
type Server struct {
	room     *room.Room
	port     int
	listener net.Listener
}

// New creates a new server.
func New(port, numTables int) *Server {
	return &Server{
		room: room.New(numTables, proto.NumCheckersPlayers),
		port: port,
	}
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer s.listener.Close()

	log.Printf("[server] listening on :%d (%d tables, %d seats/table)", s.port, s.room.NumTables(), s.room.NumSeats())
	log.Printf("[server] waiting for connections...")

	go func() {
		<-ctx.Done()
		log.Printf("[server] context cancelled, shutting down listener")
		s.listener.Close()
	}()

	for {
		raw, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Printf("[server] shutting down")
				return nil
			default:
				log.Printf("[server] accept error: %v", err)
				continue
			}
		}
		log.Printf("[server] accepted connection from %s", raw.RemoteAddr())
		go s.handleConn(raw)
	}
}

func (s *Server) handleConn(raw net.Conn) {
	addr := raw.RemoteAddr()
	log.Printf("[server] ========== new connection from %s ==========", addr)

	// Phase 1: Connection layer handshake
	log.Printf("[server] %s: phase 1 - connection handshake", addr)
	c, err := conn.ServerHandshake(raw)
	if err != nil {
		log.Printf("[server] %s: handshake FAILED: %v", addr, err)
		raw.Close()
		return
	}
	defer c.Close()
	log.Printf("[server] %s: phase 1 complete - handshake OK", addr)

	// Phase 2: Proxy negotiation
	log.Printf("[server] %s: phase 2 - proxy negotiation", addr)
	service, channel, err := proxy.Negotiate(c)
	if err != nil {
		log.Printf("[server] %s: proxy negotiation FAILED: %v", addr, err)
		return
	}
	log.Printf("[server] %s: phase 2 complete - service=%q channel=%d", addr, service, channel)

	log.Printf("[server] %s: phase 3 - room bootstrap (reading ClientConfig)", addr)
	clientCfg, err := readClientConfig(c, addr.String())
	if err != nil {
		log.Printf("[server] %s: room bootstrap FAILED: %v", addr, err)
		return
	}
	lcid, chat, skill := parseClientConfig(clientCfg)
	player := s.room.AddPlayer(c, fmt.Sprintf("Player %s", addr.String()), channel, lcid, chat, skill)
	player.Service = service
	log.Printf("[server] %s: player registered: userID=%d name=%q channel=%d lcid=%d chat=%v skill=%d",
		addr, player.UserID, player.UserName, channel, lcid, chat, skill)

	zid := proto.MarshalRoomZUserIDResponse(player.UserID, player.UserName, player.LCID)
	log.Printf("[server] player %d: sending ZUserIDResponse: data=%s", player.UserID, hex.EncodeToString(zid))
	if err := c.SendAppMessage(proto.LobbySig, channel, proto.RoomMsgZUserIDResponse, zid); err != nil {
		log.Printf("[server] player %d: send ZUserIDResponse FAILED: %v", player.UserID, err)
		return
	}

	waiting := uint32(s.room.WaitingPlayers())
	status := proto.MarshalRoomServerStatus(0, waiting)
	log.Printf("[server] player %d: sending ServerStatus: waiting=%d data=%s", player.UserID, waiting, hex.EncodeToString(status))
	if err := c.SendAppMessage(proto.LobbySig, channel, proto.RoomMsgServerStatus, status); err != nil {
		log.Printf("[server] player %d: send ServerStatus FAILED: %v", player.UserID, err)
		return
	}
	log.Printf("[server] player %d: phase 3 complete - room bootstrap done", player.UserID)

	log.Printf("[server] player %d: phase 4 - auto-matchmaking", player.UserID)
	table, seat := s.room.FindSeat(player)
	if table == nil {
		log.Printf("[server] player %d: no table available!", player.UserID)
		return
	}
	log.Printf("[server] player %d: auto-seated at table %d seat %d", player.UserID, table.ID, seat)

	if table.BothSeated() {
		gameID := s.room.NextGameID()
		log.Printf("[server] table %d: both players seated, starting game %d", table.ID, gameID)
		table.StartGame(gameID)

		players := [2]proto.RoomStartGameMPlayer{
			{UserID: table.Seats[0].UserID, LCID: table.Seats[0].LCID, Chat: table.Seats[0].Chat, Skill: table.Seats[0].Skill},
			{UserID: table.Seats[1].UserID, LCID: table.Seats[1].LCID, Chat: table.Seats[1].Chat, Skill: table.Seats[1].Skill},
		}
		for s := int16(0); s < 2; s++ {
			pl := table.Seats[s]
			if pl == nil {
				continue
			}
			startMsg := proto.MarshalRoomStartGameM(gameID, table.ID, s, players)
			log.Printf("[server] player %d: sending StartGameM: gameID=%d table=%d seat=%d data=%s",
				pl.UserID, gameID, table.ID, s, hex.EncodeToString(startMsg))
			if err := pl.Conn.SendAppMessage(proto.LobbySig, pl.Channel, proto.RoomMsgStartGameM, startMsg); err != nil {
				log.Printf("[server] player %d: send StartGameM FAILED: %v", pl.UserID, err)
			}
		}
		log.Printf("[server] game %d started on table %d", gameID, table.ID)
	} else {
		log.Printf("[server] player %d: waiting for opponent on table %d", player.UserID, table.ID)
	}

	log.Printf("[server] player %d: phase 5 - entering message loop", player.UserID)
	room.HandlePlayer(s.room, player)
	log.Printf("[server] player %d: message loop ended, connection closing", player.UserID)
}

func extractString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func readClientConfig(c *conn.Conn, addr string) (*proto.RoomClientConfig, error) {
	for round := 1; round <= 4; round++ {
		msgs, err := c.ReadAppMessages()
		if err != nil {
			return nil, err
		}
		log.Printf("[server] %s: received %d message(s) for room bootstrap round %d", addr, len(msgs), round)
		for i, m := range msgs {
			log.Printf("[server] %s: bootstrap msg[%d]: sig=%08x type=%d datalen=%d data=%s",
				addr, i, m.Signature, m.Type, len(m.Data), hex.EncodeToString(m.Data))
			if m.Signature == proto.LobbySig && m.Type == proto.RoomMsgClientConfig && len(m.Data) >= proto.RoomClientConfigSize {
				cfg := &proto.RoomClientConfig{}
				cfg.Unmarshal(m.Data)
				return cfg, nil
			}
		}
	}
	return nil, fmt.Errorf("no ClientConfig message received")
}

func parseClientConfig(cfg *proto.RoomClientConfig) (lcid uint32, chat bool, skill int16) {
	lcid = 1033
	raw := strings.TrimRight(string(cfg.Config[:]), "\x00")

	if v := extractConfigNumber(raw, "ILCID"); v != 0 {
		lcid = uint32(v)
	} else if v := extractConfigNumber(raw, "ULCID"); v != 0 {
		lcid = uint32(v)
	} else if v := extractConfigNumber(raw, "SLCID"); v != 0 {
		lcid = uint32(v)
	}

	chat = strings.Contains(raw, "Chat=<On>")
	switch {
	case strings.Contains(raw, "Skill=<Expert>"):
		skill = 2
	case strings.Contains(raw, "Skill=<Intermediate>"):
		skill = 1
	default:
		skill = 0
	}
	return lcid, chat, skill
}

func extractConfigNumber(raw, key string) int {
	marker := key + "=<"
	start := strings.Index(raw, marker)
	if start < 0 {
		return 0
	}
	start += len(marker)
	end := strings.IndexByte(raw[start:], '>')
	if end < 0 {
		return 0
	}
	n, err := strconv.Atoi(raw[start : start+end])
	if err != nil {
		return 0
	}
	return n
}

// SendRoomMsg is a helper to send a room-level message via AppMessage wrapper.
func SendRoomMsg(c *conn.Conn, channel, msgType uint32, data []byte) error {
	return c.WriteAppMessages([]wire.AppMessage{{
		Signature: proto.LobbySig,
		Channel:   channel,
		Type:      msgType,
		Data:      data,
	}})
}
