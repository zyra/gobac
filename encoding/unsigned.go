package encoding

import "bytes"

func AppendBytes(buf *bytes.Buffer, bytes []byte) error {
	_, err := buf.Write(bytes)
	return err
}

func AppendByte(buf *bytes.Buffer, b byte) error {
	return AppendBytes(buf, []byte{b})
}

func EncodeUnsigned16(buf *bytes.Buffer, value uint16) error {
	b := make([]byte, 2)
	b[0] = byte(value & 0xff00 >> 8)
	b[1] = byte(value & 0x00ff)

	return AppendBytes(buf, b)
}

func EncodeUnsigned24(buf *bytes.Buffer, value uint32) error {
	b := make([]byte, 3)
	b[0] = byte(value & 0xff0000 >> 16)
	b[1] = byte(value & 0x00ff00 >> 8)
	b[2] = byte(value & 0x0000ff)
	return AppendBytes(buf, b)
}

func EncodeUnsigned32(buf *bytes.Buffer, value uint32) error {
	b := make([]byte, 4)

	b[0] = byte((value & 0xff000000) >> 24)
	b[1] = byte((value & 0x00ff0000) >> 16)
	b[2] = byte((value & 0x0000ff00) >> 8)
	b[4] = byte(value & 0x000000ff)

	return AppendBytes(buf, b)
}

func EncodeUnsigned(buf *bytes.Buffer, value uint32) error {
	if value < 0x100 {
		return AppendByte(buf, uint8(value))
	} else if value < 0x10000 {
		return EncodeUnsigned16(buf, uint16(value))
	} else if value < 0x1000000 {
		return EncodeUnsigned24(buf, value)
	} else {
		return EncodeUnsigned32(buf, value)
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
func DecodeUnsigned(buf *bytes.Buffer, lenValue uint32) (val uint32) {
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