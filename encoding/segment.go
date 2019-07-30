package encoding

func DecodeMaxSegments(octet byte) uint8 {
	switch octet & 0xF0 {
	case 0:
		return 0
	case 0x10:
		return 2
	case 0x20:
		return 4
	case 0x30:
		return 8
	case 0x40:
		return 16
	case 0x50:
		return 32
	case 0x60:
		return 64
	case 0x70:
		return 65
	default:
		return 0
	}
}
