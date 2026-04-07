package proxy

import (
	"testing"

	"zone.com/internal/proto"
	"zone.com/internal/wire"
)

func TestConsumeProxyMessagesHiOnly(t *testing.T) {
	state := negotiationState{}
	msgs := []wire.AppMessage{{
		Signature: proto.ProxySig,
		Channel:   0,
		Type:      1,
		Data:      marshalProxyHiForTest(proto.ProxyVersion, "CHKRZM", 17311569),
	}}

	consumeProxyMessages(&state, msgs)

	if state.hi == nil {
		t.Fatal("expected ProxyHi to be parsed")
	}
	if state.svcReq != nil {
		t.Fatal("did not expect ServiceRequest")
	}
	if got := extractCString(state.hi.SetupToken[:]); got != "CHKRZM" {
		t.Fatalf("token = %q, want %q", got, "CHKRZM")
	}
}

func TestConsumeProxyMessagesServiceRequest(t *testing.T) {
	state := negotiationState{}
	msgs := []wire.AppMessage{{
		Signature: proto.ProxySig,
		Channel:   0,
		Type:      1,
		Data:      marshalProxyServiceRequestForTest(proto.ProxyRequestConnect, "mchkr_zm", 1),
	}}

	consumeProxyMessages(&state, msgs)

	if state.svcReq == nil {
		t.Fatal("expected ServiceRequest to be parsed")
	}
	if got := extractCString(state.svcReq.Service[:]); got != "mchkr_zm" {
		t.Fatalf("service = %q, want %q", got, "mchkr_zm")
	}
	if state.svcReq.Channel != 1 {
		t.Fatalf("channel = %d, want 1", state.svcReq.Channel)
	}
}

func TestConsumePackedProxyMessages(t *testing.T) {
	state := negotiationState{}
	packed := append(marshalProxyHiForTest(proto.ProxyVersion, "CHKRZM", 17311569),
		append(marshalProxyMillIDForTest(), marshalProxyServiceRequestForTest(proto.ProxyRequestConnect, "mchkr_zm", 7)...)...)

	consumeProxyMessages(&state, []wire.AppMessage{{
		Signature: proto.ProxySig,
		Channel:   0,
		Type:      3,
		Data:      packed,
	}})

	if state.hi == nil || state.svcReq == nil {
		t.Fatal("expected packed proxy payload to contain hi and service request")
	}
	if got := extractCString(state.svcReq.Service[:]); got != "mchkr_zm" {
		t.Fatalf("service = %q, want %q", got, "mchkr_zm")
	}
}

func TestPackProxyAppMessage(t *testing.T) {
	helloMsgs := helloAndSettingsPayloads()
	req := &proto.ProxyServiceRequestMsg{}
	req.Unmarshal(marshalProxyServiceRequestForTest(proto.ProxyRequestConnect, "mchkr_zm", 7))
	msg := packProxyAppMessage(0, append(helloMsgs, intakeServiceInfoPayload(req)))
	if msg.Type != 3 {
		t.Fatalf("app type = %d, want 3", msg.Type)
	}
	if msg.Channel != 0 {
		t.Fatalf("app channel = %d, want 0", msg.Channel)
	}
	submsgs, err := splitProxyPayload(msg.Data)
	if err != nil {
		t.Fatalf("splitProxyPayload: %v", err)
	}
	if len(submsgs) != 3 {
		t.Fatalf("submessage count = %d, want 3", len(submsgs))
	}
	if got := wire.ReadLE16(submsgs[0][0:]); got != uint16(proto.ProxyMsgHello) {
		t.Fatalf("first proxy type = %d, want %d", got, proto.ProxyMsgHello)
	}
	if got := wire.ReadLE16(submsgs[1][0:]); got != uint16(proto.ProxyMsgMillSettings) {
		t.Fatalf("second proxy type = %d, want %d", got, proto.ProxyMsgMillSettings)
	}
	if got := wire.ReadLE16(submsgs[2][0:]); got != uint16(proto.ProxyMsgServiceInfo) {
		t.Fatalf("third proxy type = %d, want %d", got, proto.ProxyMsgServiceInfo)
	}
}

func TestPackProxyConnectConfirmation(t *testing.T) {
	req := &proto.ProxyServiceRequestMsg{}
	req.Unmarshal(marshalProxyServiceRequestForTest(proto.ProxyRequestConnect, "mchkr_zm", 7))
	msg := packProxyAppMessage(0, [][]byte{
		connectedServiceInfoPayload(req),
		intakeServiceInfoPayload(req),
	})
	if msg.Type != 2 {
		t.Fatalf("app type = %d, want 2", msg.Type)
	}
	submsgs, err := splitProxyPayload(msg.Data)
	if err != nil {
		t.Fatalf("splitProxyPayload: %v", err)
	}
	if len(submsgs) != 2 {
		t.Fatalf("submessage count = %d, want 2", len(submsgs))
	}
	if got := wire.ReadLE32(submsgs[0][4:]); got != proto.ProxyServiceConnect {
		t.Fatalf("first reason = %d, want %d", got, proto.ProxyServiceConnect)
	}
	if got := wire.ReadLE32(submsgs[1][4:]); got != proto.ProxyServiceInfo {
		t.Fatalf("second reason = %d, want %d", got, proto.ProxyServiceInfo)
	}
}

func marshalProxyHiForTest(version uint32, token string, clientVersion uint32) []byte {
	b := make([]byte, proto.ProxyHiMsgSize)
	wire.WriteLE16(b[0:], uint16(proto.ProxyMsgHi))
	wire.WriteLE16(b[2:], proto.ProxyHiMsgSize)
	wire.WriteLE32(b[4:], version)
	copy(b[8:], append([]byte(token), 0))
	wire.WriteLE32(b[72:], clientVersion)
	return b
}

func marshalProxyServiceRequestForTest(reason uint32, service string, channel uint32) []byte {
	b := make([]byte, proto.ProxyServiceReqSize)
	wire.WriteLE16(b[0:], uint16(proto.ProxyMsgServiceRequest))
	wire.WriteLE16(b[2:], proto.ProxyServiceReqSize)
	wire.WriteLE32(b[4:], reason)
	copy(b[8:], append([]byte(service), 0))
	wire.WriteLE32(b[24:], channel)
	return b
}

func marshalProxyMillIDForTest() []byte {
	b := make([]byte, proto.ProxyMillIDMsgSize)
	wire.WriteLE16(b[0:], uint16(proto.ProxyMsgMillID))
	wire.WriteLE16(b[2:], proto.ProxyMillIDMsgSize)
	wire.WriteLE16(b[4:], 0x0409)
	wire.WriteLE16(b[6:], 0x0409)
	wire.WriteLE16(b[8:], 0x0409)
	wire.WriteLE16(b[10:], 0)
	return b
}
