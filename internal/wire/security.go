package wire

import "encoding/binary"

const (
	DefaultKey    uint32 = 0xF8273645
	ChecksumStart uint32 = 0x12344321
)

// Encrypt XORs each 4-byte block of data with the byte-swapped key.
// This matches zsecurity.c: ZEnd32(&key) then XOR each dword.
// On x86 (LE), ZEnd32 converts to big-endian, so we swap to BE before XOR.
func Encrypt(data []byte, key uint32) {
	var kb [4]byte
	binary.BigEndian.PutUint32(kb[:], key) // ZEnd32(&key)
	swapped := binary.LittleEndian.Uint32(kb[:])

	for i := 0; i+3 < len(data); i += 4 {
		v := binary.LittleEndian.Uint32(data[i:])
		binary.LittleEndian.PutUint32(data[i:], v^swapped)
	}
}

// Decrypt is identical to Encrypt (XOR is its own inverse).
func Decrypt(data []byte, key uint32) {
	Encrypt(data, key)
}

// Checksum computes the Zone checksum over data.
// Matches zsecurity.c ZSecurityGenerateChecksum:
//
//	checksum = ZEnd32(0x12344321)
//	for each dword: checksum ^= dword
//	return ZEnd32(checksum)
func Checksum(data []byte) uint32 {
	var cb [4]byte
	binary.BigEndian.PutUint32(cb[:], ChecksumStart) // ZEnd32(&checksum)
	checksum := binary.LittleEndian.Uint32(cb[:])

	for i := 0; i+3 < len(data); i += 4 {
		checksum ^= binary.LittleEndian.Uint32(data[i:])
	}

	binary.LittleEndian.PutUint32(cb[:], checksum)
	return binary.BigEndian.Uint32(cb[:]) // ZEnd32(&checksum)
}
