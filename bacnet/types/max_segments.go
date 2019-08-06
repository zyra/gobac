package types

import "github.com/kataras/iris/core/errors"

type MaxSegments uint8

func (d *MaxSegments) UnmarshalBinary(b []byte) error {
	if len(b) != 1 {
		return errors.New("max segments expects exactly 1 octet")
	}

	switch b[0] & 0xF0 {
	case 0:
		*d = 0
	case 0x10:
		*d = 2
	case 0x20:
		*d = 4
	case 0x30:
		*d = 8
	case 0x40:
		*d = 16
	case 0x50:
		*d = 32
	case 0x60:
		*d = 64
	case 0x70:
		*d = 65
	default:
		*d = 0
	}

	return nil
}
