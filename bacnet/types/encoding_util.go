package types

func EncodeVarInt(val int32) (b []byte) {
	switch GetIntLen(int(val)) {
	case 1:
		b, _ = Int8(val).MarshalBinary()
		return b

	case 2:
		b, _ = Int16(val).MarshalBinary()
		return b

	case 3:
		b, _ = Int24(val).MarshalBinary()
		return b
	default:
		b, _ = Int32(val).MarshalBinary()
		return b
	}
}

func DecodeVarInt(b []byte) int32 {
	switch len(b) {
	case 1:
		i := Int8(0)
		_ = i.UnmarshalBinary(b)
		return int32(i)

	case 2:
		i := Int16(0)
		_ = i.UnmarshalBinary(b)
		return int32(i)

	case 3:
		i := Int24(0)
		_ = i.UnmarshalBinary(b)
		return int32(i)
	default:
		i := Int32(0)
		_ = i.UnmarshalBinary(b)
		return int32(i)
	}
}

func EncodeVarUint(value uint32) (b []byte) {
	switch GetUintLen(uint(value)) {
	case 1:
		return []byte{uint8(value)}

	case 2:
		b, _ = Uint16(value).MarshalBinary()
		return b

	case 3:
		b, _ = Uint24(value).MarshalBinary()
		return b
	default:
		b, _ = Uint32(value).MarshalBinary()
		return b
	}
}

func DecodeVarUint(b []byte) uint32 {
	switch len(b) {
	case 0:
		return 0
	case 1:
		return uint32(b[0])

	case 2:
		v := Uint16(0)
		_ = v.UnmarshalBinary(b)
		return uint32(v)

	case 3:
		v := Uint24(0)
		_ = v.UnmarshalBinary(b)
		return uint32(v)

	default:
		v := Uint32(0)
		_ = v.UnmarshalBinary(b)
		return uint32(v)
	}
}
