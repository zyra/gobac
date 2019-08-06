package types

import "errors"

type MaxApduValue uint16

func (v *MaxApduValue) UnmarshalBinary(b []byte) error {
	if len(b) != 1 {
		return errors.New("maxApdu expects one octet")
	}

	switch b[0] & 0x0F {
	case 0:
		*v = 50
	case 1:
		*v = 128
	case 2:
		*v = 206
	case 3:
		*v = 480
	case 4:
		*v = 1024
	case 5:
		*v = 1476
	default:
		*v = 0
	}

	return nil
}
