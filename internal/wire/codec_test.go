package wire

import (
	"bytes"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	key := uint32(0x42424242)
	seq := uint32(1)

	msgs := []AppMessage{
		{Signature: 0x726F7574, Channel: 0, Type: 1, Data: []byte{0xAA, 0xBB, 0xCC, 0xDD}},
		{Signature: 0x726F7574, Channel: 0, Type: 2, Data: []byte{0x11, 0x22}},
	}

	// Write
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf, key, seq)
	if err := fw.WriteGeneric(msgs); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read
	fr := NewFrameReader(&buf, key, seq)
	got, err := fr.ReadGeneric()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(got) != len(msgs) {
		t.Fatalf("got %d messages, want %d", len(got), len(msgs))
	}

	for i := range msgs {
		if got[i].Signature != msgs[i].Signature {
			t.Errorf("msg[%d] sig: got %08x, want %08x", i, got[i].Signature, msgs[i].Signature)
		}
		if got[i].Type != msgs[i].Type {
			t.Errorf("msg[%d] type: got %d, want %d", i, got[i].Type, msgs[i].Type)
		}
		if !bytes.Equal(got[i].Data, msgs[i].Data) {
			t.Errorf("msg[%d] data: got %x, want %x", i, got[i].Data, msgs[i].Data)
		}
	}
}

func TestCodecNoEncryption(t *testing.T) {
	msgs := []AppMessage{
		{Signature: 0x67616D65, Channel: 0, Type: 9, Data: make([]byte, 12)},
	}

	var buf bytes.Buffer
	fw := NewFrameWriter(&buf, 0, 1)
	if err := fw.WriteGeneric(msgs); err != nil {
		t.Fatalf("write: %v", err)
	}

	fr := NewFrameReader(&buf, 0, 1)
	got, err := fr.ReadGeneric()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1", len(got))
	}
}

func TestHelloMarshalSize(t *testing.T) {
	msg := &HelloMsg{
		FirstSequenceID: 1,
		Key:             0x42424242,
		OptionFlags:     OptionAggGeneric,
	}
	b := msg.Marshal()
	if len(b) != HelloMsgSize {
		t.Fatalf("HelloMsg marshal: got %d bytes, want %d", len(b), HelloMsgSize)
	}
}
