package encoding

import (
	"encoding/binary"
	"math"
)

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

func (buf *Buffer) EncodeSigned8(val int8) error {
	b := make([]byte, 1)
	binary.PutVarint(b, int64(val))
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeSigned16(val int16) error {
	b := make([]byte, 2)
	binary.PutVarint(b, int64(val))
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeSigned24(val int32) error {
	b := make([]byte, 3)
	binary.PutVarint(b, int64(val))
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeSigned32(val int32) error {
	b := make([]byte, 4)
	binary.PutVarint(b, int64(val))
	return buf.AppendBytes(b)
}

func (buf *Buffer) EncodeSigned(val int32) error {
	if val <= math.MaxInt8 && val >= math.MinInt16 {
		return buf.EncodeSigned8(int8(val))
	}

	if val <= math.MaxInt16 && val >= math.MinInt16 {
		return buf.EncodeSigned16(int16(val))
	}

	if val <= 8388607 && val >= -8388607 {
		return buf.EncodeSigned24(val)
	}

	return buf.EncodeSigned32(val)
}
