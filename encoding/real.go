package encoding

import (
	"encoding/binary"
	"math"
)

func (buf *Buffer) DecodeReal(lenValue uint32) (val float32) {
	if lenValue != 4 {
		return 0
	}

	bits := binary.LittleEndian.Uint32(buf.Next(4))
	return math.Float32frombits(bits)
}
