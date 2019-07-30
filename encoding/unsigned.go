package encoding

import (
	"encoding/binary"
	"math"
)

func (buf *Buffer) EncodeUnsigned16(value uint16) error {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, value)
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned24(value uint32) error {
	b := make([]byte, 3)
	b[0] = byte(value >> 24)
	b[1] = byte(value >> 16)
	b[2] = byte(value >> 8)
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned32(value uint32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, value)
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned(value uint32) error {
	if value <= math.MaxUint8 {
		return buf.AppendByte(uint8(value))
	} else if value <= math.MaxUint16 {
		return buf.EncodeUnsigned16(uint16(value))
	} else if value <= 1<<24-1 {
		return buf.EncodeUnsigned24(value)
	} else {
		return buf.EncodeUnsigned32(value)
	}
}

// Decodes unsigned 16 value
// Takes 2 bytes
func (buf *Buffer) DecodeUnsigned16() (val uint16) {
	return binary.BigEndian.Uint16(buf.Next(2))
}

// Decodes unsigned 16 value
// Takes 2 bytes
func DecodeUnsigned16(data []byte) (val uint16) {
	return binary.BigEndian.Uint16(data)
}

// Decodes unsigned 24 value
// Takes 3 bytes
func DecodeUnsigned24(data []byte) (val uint32) {
	val |= (uint32(data[0]) << 16) & 0x00FF0000
	val |= (uint32(data[1]) << 8) & 0x0000FF00
	val |= uint32(data[2]) & 0x000000FF
	return val
}

// Decodes unsigned 32 value
// Takes 4 bytes
func DecodeUnsigned32(data []byte) (val uint32) {
	return binary.BigEndian.Uint32(data)
}

// Decodes unsigned value
func (buf *Buffer) DecodeUnsigned(lenValue uint32) (val uint32) {
	switch lenValue {
	case 1:
		return uint32(buf.NextOne())

	case 2:
		return uint32(DecodeUnsigned16(buf.Next(2)))

	case 3:
		return DecodeUnsigned24(buf.Next(3))

	case 4:
		return DecodeUnsigned32(buf.Next(4))

	default:
		return 0
	}
}
