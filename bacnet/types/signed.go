package types

import (
	"encoding/binary"
	"errors"
)

type Int8 int8

func (i Int8) MarshalBinary() (b []byte, e error) {
	return []byte{byte(i)}, nil
}

func (i *Int8) UnmarshalBinary(b []byte) error {
	if len(b) != 1 {
		return errors.New("int8 expects exactly 1 octet")
	}
	*i = Int8(int8(b[0]))
	return nil
}

type Int16 int16

func (i Int16) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(i))
	return b, nil
}

func (i *Int16) UnmarshalBinary(b []byte) error {
	if len(b) != 2 {
		return errors.New("int16 expects exactly 2 octets")
	}
	*i = Int16(int16(binary.BigEndian.Uint16(b)))
	return nil
}

type Int24 int32

func (i Int24) MarshalBinary() (b []byte, e error) {
	v := uint32(i)
	return []byte{byte(v >> 16), byte(v >> 8), byte(v)}, nil
}

func (i *Int24) UnmarshalBinary(b []byte) error {
	if len(b) != 3 {
		return errors.New("int24 expects exactly 3 octets")
	}
	v := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	if b[0]&0x80 != 0 {
		v |= 0xff000000
	}
	*i = Int24(int32(v))
	return nil
}

type Int32 int32

func (i Int32) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(i))
	return b, nil
}

func (i *Int32) UnmarshalBinary(b []byte) error {
	if len(b) != 4 {
		return errors.New("int32 expects exactly 4 octets")
	}
	*i = Int32(int32(binary.BigEndian.Uint32(b)))
	return nil
}
