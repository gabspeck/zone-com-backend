package wire

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	orig := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	data := make([]byte, len(orig))
	copy(data, orig)

	key := uint32(0xAABBCCDD)
	Encrypt(data, key)
	if bytes.Equal(data, orig) {
		t.Fatal("encrypt should change data")
	}
	Decrypt(data, key)
	if !bytes.Equal(data, orig) {
		t.Fatalf("round-trip failed: got %x, want %x", data, orig)
	}
}

func TestEncryptDecryptDefaultKey(t *testing.T) {
	orig := []byte{
		0x4C, 0x69, 0x4E, 0x6B, // 'LiNk'
		0x30, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x0C, 0x00,
		0x03, 0x00, 0x00, 0x00,
	}
	data := make([]byte, len(orig))
	copy(data, orig)

	Encrypt(data, DefaultKey)
	if bytes.Equal(data, orig) {
		t.Fatal("encrypt with default key should change data")
	}
	Decrypt(data, DefaultKey)
	if !bytes.Equal(data, orig) {
		t.Fatalf("default key round-trip failed: got %x, want %x", data, orig)
	}
}

func TestChecksumDeterministic(t *testing.T) {
	data := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	c1 := Checksum(data)
	c2 := Checksum(data)
	if c1 != c2 {
		t.Fatalf("checksum not deterministic: %08x != %08x", c1, c2)
	}
}

func TestChecksumZeroData(t *testing.T) {
	data := make([]byte, 8)
	c := Checksum(data)
	// XOR with all zeros = ChecksumStart itself (after double swap)
	if c != ChecksumStart {
		t.Fatalf("checksum of zeros: got %08x, want %08x", c, ChecksumStart)
	}
}
