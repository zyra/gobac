package types

import (
	"encoding/binary"
	"github.com/kataras/iris/core/errors"
	"math"
)

type Real float32

func (r Real) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 4)
	binary.BigEndian.PutUint32(b, math.Float32bits(float32(r)))
	return b, nil
}

func (r *Real) UnmarshalBinary(b []byte) error {
	if len(b) != 4 {
		return errors.New("real expects 4 octets")
	}
	u := binary.BigEndian.Uint32(b)
	*r = Real(math.Float32frombits(u))
	return nil
}
