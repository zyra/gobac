package types

type Tag struct {
	TagNumber uint8
	LenValue  int
}

func (ct *Tag) Is(n uint8) bool {
	return ct.TagNumber == n
}
func (ct *Tag) Isnt(n uint8) bool {
	return ct.TagNumber != n
}

func (ct *Tag) Reset() {
	ct.TagNumber = 0
	ct.LenValue = 0
}

func (ct *Tag) encodeTagCommonBase(b []byte) {
	if ct.TagNumber <= 14 {
		b[0] |= byte(ct.TagNumber) << 4
	} else {
		if len(b) == 1 {
			b = append(b, byte(0))
		}

		b[0] |= 0xF0
		b[1] = byte(ct.TagNumber)
	}
}

func (ct *Tag) encodeTagCommonExtended(b []byte) {
	if ct.LenValue <= 4 {
		b[0] |= byte(ct.LenValue)
		return
	}

	if len(b) == 1 {
		b = append(b, byte(0))
	}

	if ct.LenValue <= 253 {
		b[1] = byte(ct.LenValue)
	} else if ct.LenValue <= 65535 {
		b[1] = 254
		copy(b[2:], EncodeVarUint(uint32(ct.LenValue)))
	} else {
		b[1] = 255
		copy(b[2:], EncodeVarUint(uint32(ct.LenValue)))
	}
}

func (ct *Tag) encodeContextTag() (b []byte) {
	b = []byte{0x08}
	ct.encodeTagCommonBase(b)
	return b
}

func (ct *Tag) EncodeOpeningTag() (b []byte) {
	b = ct.encodeContextTag()
	b[0] |= 6
	return b
}

func (ct *Tag) EncodeClosingTag() (b []byte) {
	b = ct.encodeContextTag()
	b[0] |= 7
	return b
}

func (ct *Tag) EncodeContextTag() (b []byte) {
	b = ct.encodeContextTag()
	ct.encodeTagCommonExtended(b)
	return b
}

func (ct *Tag) EncodeTag() (b []byte) {
	b = []byte{0}
	ct.encodeTagCommonBase(b)
	ct.encodeTagCommonExtended(b)
	return b
}

func (ct *Tag) DecodeTag(b []byte) (bytesRead int) {
	if len(b) == 0 {
		return 0
	}

	b1 := b[0]
	var b2 byte
	bytesRead = 1

	if b1&0xF0 == 0xF0 {
		bytesRead++
		b2 = b[1]
		ct.TagNumber = b2
	} else {
		ct.TagNumber = b1 >> 4
	}

	switch b1 & 0x07 {
	case 5:
		switch b2 {
		case 255:
			v := Uint32(0)
			_ = v.UnmarshalBinary(b[bytesRead:])
			ct.LenValue = int(v)
			bytesRead += 4
			break
		case 254:
			v := Uint16(0)
			_ = v.UnmarshalBinary(b[bytesRead:])
			ct.LenValue = int(v)
			bytesRead += 2
			break
		default:
			ct.LenValue = int(b[bytesRead])
			bytesRead++
		}
		break

	case 6, 7:
		ct.LenValue = 0
		break

	default:
		ct.LenValue = int(uint32(b1) & 0x07)
	}

	return bytesRead
}
