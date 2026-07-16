package types

import (
	"errors"
	"math/bits"
)

type BitString []byte

func (bs BitString) MarshalBinary() ([]byte, error) {
	out := make([]byte, len(bs)+1)
	// BitString represents complete octets, so the unused-bit count is zero.
	for i, b := range bs {
		out[i+1] = bits.Reverse8(b)
	}

	return out, nil
}

func (bs *BitString) UnmarshalBinary(b []byte) error {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	out := make([]byte, len(b)-1)

	if b[0] > 7 {
		return errors.New("bit string unused-bit count exceeds seven")
	}
	for i, b := range b[1:] {
		out[i] = bits.Reverse8(b)
	}

	*bs = out

	return nil
}
