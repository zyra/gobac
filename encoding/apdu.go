package encoding

func DecodeMaxAPDU(octet byte) uint16 {
	switch octet & 0x0F {
	case 0:
		return 50
	case 1:
		return 128
	case 2:
		return 206
	case 3:
		return 480
	case 4:
		return 1024
	case 5:
		return 1476
	default:
		return 0
	}
}
