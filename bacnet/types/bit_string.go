package types

import (
	"math/bits"
)

type BitString []byte

func (bs BitString) MarshalBinary() ([]byte, error) {
	out := make([]byte, len(bs)+1)
	// TODO handle bits used
	for i, b := range bs {
		out[i] = bits.Reverse8(b)
	}

	return out, nil
}

func (bs *BitString) UnmarshalBinary(b []byte) error {
	out := make([]byte, len(b)-1)

	// TODO handle bits used
	for i, b := range b[1:] {
		out[i] = bits.Reverse8(b)
	}

	*bs = out

	return nil
}
