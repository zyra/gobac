package util

import "bytes"

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