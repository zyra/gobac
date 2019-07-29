package encoding

import "bytes"

func EncodeTag(buf *bytes.Buffer, tag uint8, contextSpecific bool, lenValueType uint32) error {
	b := []byte{0}
	var err error

	if contextSpecific {
		b[0] = 0x08
	}

	if tag <= 14 {
		b[0] |= tag << 4
	} else {
		b[0] |= 0xF0
		b[1] = tag
	}

	if lenValueType <= 4 {
		b[0] |= byte(lenValueType)
	} else {
		b[0] |= 5

		if lenValueType <= 253 {
			b[1] = byte(lenValueType)
		} else if lenValueType <= 65535 {
			b[1] = 254
			defer func() {
				err = EncodeUnsigned16(buf, uint16(lenValueType))
			}()
		} else {
			b[1] = 255
			defer func() {
				err = EncodeUnsigned(buf, lenValueType)
			}()
		}
	}

	err = AppendBytes(buf, b)

	return err
}

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
