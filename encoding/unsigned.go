package encoding

import "bytes"

type Buffer struct {
	*bytes.Buffer
}

func NewBuffer(data ...[]byte) *Buffer {
	var b []byte

	if len(data) == 1 {
		b = data[0]
	} else {
		b = make([]byte, 0)
	}

	return &Buffer{
		Buffer: bytes.NewBuffer(b),
	}
}

func (buf *Buffer) AppendBytes(bytes []byte) error {
	_, err := buf.Write(bytes)
	return err
}

func (buf *Buffer) AppendByte(b byte) error {
	return buf.AppendBytes([]byte{b})
}

func (buf *Buffer) EncodeUnsigned16(value uint16) error {
	b := make([]byte, 2)
	b[0] = byte(value & 0xff00 >> 8)
	b[1] = byte(value & 0x00ff)

	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned24(value uint32) error {
	b := make([]byte, 3)
	b[0] = byte(value & 0xff0000 >> 16)
	b[1] = byte(value & 0x00ff00 >> 8)
	b[2] = byte(value & 0x0000ff)
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned32(value uint32) error {
	b := make([]byte, 4)

	b[0] = byte((value & 0xff000000) >> 24)
	b[1] = byte((value & 0x00ff0000) >> 16)
	b[2] = byte((value & 0x0000ff00) >> 8)
	b[4] = byte(value & 0x000000ff)

	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeUnsigned(value uint32) error {
	if value < 0x100 {
		return buf.AppendByte(uint8(value))
	} else if value < 0x10000 {
		return buf.EncodeUnsigned16(uint16(value))
	} else if value < 0x1000000 {
		return buf.EncodeUnsigned24(value)
	} else {
		return buf.EncodeUnsigned32(value)
	}
}

// Decodes unsigned 16 value
// Takes 2 bytes
func DecodeUnsigned16(data []byte) (val uint16) {
	val = ((uint16(data[0])) << 8) & 0xFF00
	val |= uint16(data[1]) & 0x00FF
	return val
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
	val = uint32((uint32(data[0]) << 24) & 0xFF000000)
	val |= uint32((uint32(data[1]) << 16) & 0x00FF0000)
	val |= uint32((uint32(data[2]) << 8) & 0x0000FF00)
	val |= uint32(uint32(data[3]) & 0x000000FF)

	return val
}

// Decodes unsigned value
func (buf *Buffer) DecodeUnsigned(lenValue uint32) (val uint32) {
	switch lenValue {
	case 1:
		return uint32(buf.Next(1)[0])

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
