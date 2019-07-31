package encoding

import (
	"encoding/binary"
	"math"
)

func (buf *Buffer) DecodeReal(lenValue uint32) (val float32) {
	if lenValue != 4 {
		return 0
	}

	u := binary.BigEndian.Uint32(buf.Next(4))
	return math.Float32frombits(u)
}

func (buf *Buffer) EncodeReal(value float32) error {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, math.Float32bits(value))
	return buf.AppendBytes(bytes)
}
