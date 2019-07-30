package encoding

import "encoding/binary"

func (buf *Buffer) DecodeSigned8() (val int8) {
	v, _ := binary.Varint(buf.Next(1))
	return int8(v)
}

func (buf *Buffer) DecodeSigned16() (val int16) {
	v, _ := binary.Varint(buf.Next(2))
	return int16(v)
}

func (buf *Buffer) DecodeSigned24() (val int32) {
	v, _ := binary.Varint(buf.Next(3))
	return int32(v)
}

func (buf *Buffer) DecodeSigned32() (val int32) {
	v, _ := binary.Varint(buf.Next(4))
	return int32(v)
}

func (buf *Buffer) DecodeSigned(lenValue uint32) (val int32) {
	switch lenValue {
	case 1:
		return int32(buf.DecodeSigned8())
	case 2:
		return int32(buf.DecodeSigned16())
	case 3:
		return buf.DecodeSigned24()
	case 4:
		return buf.DecodeSigned32()
	default:
		return 0
	}
}
