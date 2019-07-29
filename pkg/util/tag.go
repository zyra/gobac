package util

import "bytes"

func DecodeTag(buf *bytes.Buffer) (tagNumber uint8, lenValue uint32) {
	b1, _ := buf.ReadByte()

	if (b1 & 0xF0) == 0xF0 {
		b2, _ := buf.ReadByte()
		tagNumber = b2
	} else {
		tagNumber = b1 >> 4
	}

	if b1 & 0x07 == 5 {
		n := buf.Bytes()[0]
		switch n {
		case 255:
			lenValue = DecodeUnsigned32(buf.Next(4))
			break
		case 254:
			lenValue = uint32(DecodeUnsigned16(buf.Next(2)))
			break
		default:
			lenValue = uint32(buf.Next(1)[0])
		}
	} else if b1&0x07 == 6 {
		lenValue = 0
	} else if b1&0x07 == 7 {
		lenValue = 0
	} else {
		lenValue = uint32(b1) & 0x07
	}

	return tagNumber, lenValue
}
