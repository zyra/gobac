package pdu

import (
	"bytes"
	"fmt"
)

type Buffer struct {
	*bytes.Buffer
}

func NewBuffer(bas ...[]byte) *Buffer {
	var ba []byte

	if len(bas) == 0 {
		ba = make([]byte, 0)
	} else {
		ba = bas[0]
	}

	b := &Buffer{
		Buffer: bytes.NewBuffer(ba),
	}

	return b
}

func (d *Buffer) Append(bytes ...byte) {
	d.AppendArray(bytes)
}

func (d *Buffer) AppendArray(bytes []byte) {
	if _, err := d.Write(bytes); err != nil {
		fmt.Println("Error appending bytes: ", err)
	}
}

func (d *Buffer) EncodeUnsigned16(value uint16) {
	b := make([]byte, 2)
	b[0] = byte(value & 0xff00 >> 8)
	b[1] = byte(value & 0x00ff)
	d.AppendArray(b)
}

func (d *Buffer) EncodeUnsigned24(value uint32) {
	b := make([]byte, 3)
	b[0] = byte(value & 0xff0000 >> 16)
	b[1] = byte(value & 0x00ff00 >> 8)
	b[2] = byte(value & 0x0000ff)
	d.AppendArray(b)
}

func (d *Buffer) EncodeUnsigned32(value uint32) {
	b := make([]byte, 4)

	b[0] = byte((value & 0xff000000) >> 24)
	b[1] = byte((value & 0x00ff0000) >> 16)
	b[2] = byte((value & 0x0000ff00) >> 8)
	b[4] = byte(value & 0x000000ff)

	d.AppendArray(b)
}

func (d *Buffer) EncodeUnsigned(value uint32) {
	if value < 0x100 {
		d.Append(uint8(value))
	} else if value < 0x10000 {
		d.EncodeUnsigned16(uint16(value))
	} else if value < 0x1000000 {
		d.EncodeUnsigned24(value)
	} else {
		d.EncodeUnsigned32(value)
	}
}

func (d *Buffer) EncodeTag(tag uint8, contextSpecific bool, lenValueType uint32) {
	b := []byte{0}

	if contextSpecific {
		b[0] = 0x08
	}

	if tag <= 14 {
		b[0] |= tag << 4
	} else {
		b[0] |= 0xF0
		b[1] = tag
	}

	if lenValueType <= 4 {
		b[0] |= byte(lenValueType)
	} else {
		b[0] |= 5

		if lenValueType <= 253 {
			b[1] = byte(lenValueType)
		} else if lenValueType <= 65535 {
			b[1] = 254
			defer d.EncodeUnsigned16(uint16(lenValueType))
		} else {
			b[1] = 255
			defer d.EncodeUnsigned(lenValueType)
		}
	}

	d.AppendArray(b)
}
