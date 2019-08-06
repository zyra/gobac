package types

import "encoding/binary"

type Int8 int8

func (i Int8) MarshalBinary() (b []byte, e error) {
	b = []byte{0,0}
	binary.PutVarint(b, int64(i))
	return b, nil
}

func (i *Int8) UnmarshalBinary(b []byte) error {
	v, _ := binary.Varint(b)
	*i = Int8(v)
	return nil
}

type Int16 int16

func (i Int16) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 2)
	binary.PutVarint(b, int64(i))
	return b, nil
}

func (i *Int16) UnmarshalBinary(b []byte) error {
	v, _ := binary.Varint(b)
	*i = Int16(v)
	return nil
}

type Int24 int32

func (i Int24) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 3)
	binary.PutVarint(b, int64(i))
	return b, nil
}

func (i *Int24) UnmarshalBinary(b []byte) error {
	v, _ := binary.Varint(b)
	*i = Int24(v)
	return nil
}

type Int32 int32

func (i Int32) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 4)
	binary.PutVarint(b, int64(i))
	return b, nil
}

func (i *Int32) UnmarshalBinary(b []byte) error {
	v, _ := binary.Varint(b)
	*i = Int32(v)
	return nil
}
