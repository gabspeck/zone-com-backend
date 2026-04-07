package wire

import "encoding/binary"

// Little-endian helpers (connection layer, proxy, room fields)

func ReadLE16(b []byte) uint16  { return binary.LittleEndian.Uint16(b) }
func ReadLE32(b []byte) uint32  { return binary.LittleEndian.Uint32(b) }
func WriteLE16(b []byte, v uint16) { binary.LittleEndian.PutUint16(b, v) }
func WriteLE32(b []byte, v uint32) { binary.LittleEndian.PutUint32(b, v) }

// Big-endian helpers (checkers game fields, ZEnd16/ZEnd32 equivalents)

func ReadBE16(b []byte) uint16  { return binary.BigEndian.Uint16(b) }
func ReadBE32(b []byte) uint32  { return binary.BigEndian.Uint32(b) }
func WriteBE16(b []byte, v uint16) { binary.BigEndian.PutUint16(b, v) }
func WriteBE32(b []byte, v uint32) { binary.BigEndian.PutUint32(b, v) }

// SwapEnd32 performs ZEnd32: byte-swap a uint32 (LE<->BE toggle).
func SwapEnd32(v uint32) uint32 {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return binary.BigEndian.Uint32(b[:])
}
