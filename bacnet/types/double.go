package types

import (
	"encoding/binary"
	"errors"
	"math"
)

type Double float64

func (d Double) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 8)
	bits := math.Float64bits(float64(d))
	binary.BigEndian.PutUint64(b, bits)
	return b, e
}

func (d *Double) UnmarshalBinary(b []byte) error {
	if len(b) != 8 {
		return errors.New("double expects 8 octets")
	}

	bits := binary.BigEndian.Uint64(b)
	*d = Double(math.Float64frombits(bits))

	return nil
}
