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
