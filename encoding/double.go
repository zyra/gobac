package encoding

import (
	"encoding/binary"
	"math"
)

func (buf *Buffer) DecodeDouble(lenValue uint32) (val float64) {
	if lenValue != 8 {
		return 0
	}

	bits := binary.LittleEndian.Uint64(buf.Next(8))
	return math.Float64frombits(bits)
}

func (buf *Buffer) EncodeDouble(value float64) error {
	bytes := make([]byte, 8)
	bits := math.Float64bits(value)
	binary.LittleEndian.PutUint64(bytes, bits)
	return buf.AppendBytes(bytes)
}
