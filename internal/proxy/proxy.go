package proxy

import (
	"fmt"
	"log"

	"zone.com/internal/conn"
	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

type negotiationState struct {
	hi     *proto.ProxyHiMsg
	svcReq *proto.ProxyServiceRequestMsg
}

// Negotiate handles the proxy protocol phase after the connection handshake.
// Some clients send ProxyHi + MillID + ServiceRequest in one Generic frame,
// while others send ProxyHi first and wait for a response before sending the
// ServiceRequest. Support both forms.
// Returns the requested service name and channel.
func Negotiate(c *conn.Conn) (service string, channel uint32, err error) {
	log.Printf("[proxy] reading client proxy messages...")

	state := negotiationState{}
	sentHello := false
	sentConnect := false

	for round := 1; round <= 4; round++ {
		msgs, err := c.ReadAppMessages()
		if err != nil {
			return "", 0, fmt.Errorf("proxy read: %w", err)
		}

		log.Printf("[proxy] round %d: received %d proxy message(s)", round, len(msgs))
		consumeProxyMessages(&state, msgs)

		if state.hi == nil {
			continue
		}
		if state.hi.ProtocolVersion != proto.ProxyVersion {
			return "", 0, fmt.Errorf("proxy: bad version %d (want %d)", state.hi.ProtocolVersion, proto.ProxyVersion)
		}

		if !sentHello {
			responseMsgs := helloAndSettingsPayloads()
			if state.svcReq != nil {
				responseMsgs = append(responseMsgs, intakeServiceInfoPayload(state.svcReq))
			}
			packed := packProxyAppMessage(0, responseMsgs)
			log.Printf("[proxy] sending response: %d packed proxy submessage(s)", packed.Type)
			if err := c.WriteAppMessages([]wire.AppMessage{packed}); err != nil {
				return "", 0, fmt.Errorf("proxy write response: %w", err)
			}
			sentHello = true
		}

		if state.svcReq != nil && !sentConnect {
			connected := packProxyAppMessage(0, [][]byte{
				connectedServiceInfoPayload(state.svcReq),
				intakeServiceInfoPayload(state.svcReq),
			})
			log.Printf("[proxy] sending connect confirmation: %d packed proxy submessage(s)", connected.Type)
			if err := c.WriteAppMessages([]wire.AppMessage{connected}); err != nil {
				return "", 0, fmt.Errorf("proxy write connect confirmation: %w", err)
			}
			sentConnect = true
		}

		if state.svcReq != nil && sentConnect {
			svcName := extractCString(state.svcReq.Service[:])
			log.Printf("[proxy] negotiation complete: service=%q channel=%d", svcName, state.svcReq.Channel)
			return svcName, state.svcReq.Channel, nil
		}
	}

	if state.hi == nil {
		return "", 0, fmt.Errorf("proxy: no Hi message received")
	}
	return "", 0, fmt.Errorf("proxy: no ServiceRequest message received")
}

func extractCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func consumeProxyMessages(state *negotiationState, msgs []wire.AppMessage) {
	for i, m := range msgs {
		log.Printf("[proxy] msg[%d]: sig=%08x ch=%d type=%d datalen=%d", i, m.Signature, m.Channel, m.Type, len(m.Data))
		if m.Signature != proto.ProxySig {
			log.Printf("[proxy]   skipping non-proxy message (sig=%08x)", m.Signature)
			continue
		}
		submsgs, err := splitProxyPayload(m.Data)
		if err != nil {
			log.Printf("[proxy]   invalid proxy payload: %v", err)
			continue
		}
		for j, sub := range submsgs {
			ptype := wire.ReadLE16(sub[0:])
			plength := wire.ReadLE16(sub[2:])
			log.Printf("[proxy]   submsg[%d]: proxy type=%d length=%d", j, ptype, plength)
			switch ptype {
			case uint16(proto.ProxyMsgHi):
				state.hi = &proto.ProxyHiMsg{}
				state.hi.Unmarshal(sub)
				token := extractCString(state.hi.SetupToken[:])
				log.Printf("[proxy]   ProxyHi: ver=%d clientVer=%d token=%q",
					state.hi.ProtocolVersion, state.hi.ClientVersion, token)
			case uint16(proto.ProxyMsgMillID):
				var millID proto.ProxyMillIDMsg
				millID.Unmarshal(sub)
				log.Printf("[proxy]   MillID: sysLang=%d userLang=%d appLang=%d tzMinutes=%d",
					millID.SysLang, millID.UserLang, millID.AppLang, millID.TimeZoneMinutes)
			case uint16(proto.ProxyMsgServiceRequest):
				state.svcReq = &proto.ProxyServiceRequestMsg{}
				state.svcReq.Unmarshal(sub)
				svc := extractCString(state.svcReq.Service[:])
				log.Printf("[proxy]   ServiceRequest: reason=%d service=%q channel=%d",
					state.svcReq.Reason, svc, state.svcReq.Channel)
			default:
				log.Printf("[proxy]   unknown proxy type %d", ptype)
			}
		}
	}
}

func helloAndSettingsPayloads() [][]byte {
	helloData := proto.MarshalProxyHello()
	settingsData := proto.MarshalProxyMillSettings(proto.ProxyMillChatFull, proto.ProxyMillStatsAll)
	return [][]byte{helloData, settingsData}
}

func intakeServiceInfoPayload(svcReq *proto.ProxyServiceRequestMsg) []byte {
	svcName := extractCString(svcReq.Service[:])
	log.Printf("[proxy] client wants: service=%q channel=%d reason=%d", svcName, svcReq.Channel, svcReq.Reason)
	return proto.MarshalProxyServiceInfo(
		proto.ProxyServiceInfo,
		svcReq.Service,
		svcReq.Channel,
		proto.ProxyServiceAvailable|proto.ProxyServiceLocal|proto.ProxyServiceConnected,
		[4]byte{127, 0, 0, 1},
		proto.PortMillenniumProxy,
	)
}

func connectedServiceInfoPayload(svcReq *proto.ProxyServiceRequestMsg) []byte {
	return proto.MarshalProxyServiceInfo(
		proto.ProxyServiceConnect,
		svcReq.Service,
		svcReq.Channel,
		proto.ProxyServiceAvailable|proto.ProxyServiceLocal|proto.ProxyServiceConnected,
		[4]byte{127, 0, 0, 1},
		proto.PortMillenniumProxy,
	)
}

func packProxyAppMessage(channel uint32, payloads [][]byte) wire.AppMessage {
	totalLen := 0
	for _, payload := range payloads {
		totalLen += len(payload)
	}
	data := make([]byte, 0, totalLen)
	for _, payload := range payloads {
		data = append(data, payload...)
	}
	return wire.AppMessage{
		Signature: proto.ProxySig,
		Channel:   channel,
		Type:      uint32(len(payloads)),
		Data:      data,
	}
}

func splitProxyPayload(data []byte) ([][]byte, error) {
	var out [][]byte
	for len(data) > 0 {
		if len(data) < proto.ProxyHeaderSize {
			return nil, fmt.Errorf("proxy payload truncated: need at least %d bytes, have %d", proto.ProxyHeaderSize, len(data))
		}
		msgLen := int(wire.ReadLE16(data[2:]))
		if msgLen < proto.ProxyHeaderSize {
			return nil, fmt.Errorf("proxy submessage too short: %d", msgLen)
		}
		if msgLen > len(data) {
			return nil, fmt.Errorf("proxy submessage length %d exceeds remaining payload %d", msgLen, len(data))
		}
		out = append(out, data[:msgLen])
		data = data[msgLen:]
	}
	return out, nil
}
