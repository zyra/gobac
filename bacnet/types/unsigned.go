package types

import (
	"encoding/binary"
	"errors"
)

type Uint16 uint16

func (u Uint16) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(u))
	return b, nil
}

func (u *Uint16) UnmarshalBinary(b []byte) error {
	if len(b) != 2 {
		return errors.New("uint16 expects exactly 2 octets")
	}
	*u = Uint16(binary.BigEndian.Uint16(b))
	return nil
}

type Uint24 uint32

func (u Uint24) MarshalBinary() (b []byte, e error) {
	return []byte{
		byte(u >> 16),
		byte(u >> 8),
		byte(u),
	}, nil
}

func (u *Uint24) UnmarshalBinary(b []byte) error {
	if len(b) != 3 {
		return errors.New("uint24 expects exactly 3 octets")
	}
	val := uint32(0)
	val |= (uint32(b[0]) << 16) & 0x00FF0000
	val |= (uint32(b[1]) << 8) & 0x0000FF00
	val |= uint32(b[2]) & 0x000000FF
	*u = Uint24(val)
	return nil
}

type Uint32 uint32

func (u Uint32) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(u))
	return b, nil
}

func (u *Uint32) UnmarshalBinary(b []byte) error {
	if len(b) != 4 {
		return errors.New("uint32 expects exactly 4 octets")
	}
	*u = Uint32(binary.BigEndian.Uint32(b))
	return nil
}
