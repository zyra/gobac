package encoding

import "bytes"

type Buffer struct {
	*bytes.Buffer
}

func NewBuffer(data ...[]byte) *Buffer {
	var b []byte

	if len(data) == 1 {
		b = data[0]
	} else {
		b = make([]byte, 0)
	}

	return &Buffer{
		Buffer: bytes.NewBuffer(b),
	}
}

func (buf *Buffer) NextOne() byte {
	b, _ := buf.ReadByte()
	return b
}

func (buf *Buffer) AppendBytes(bytes []byte) error {
	_, err := buf.Write(bytes)
	return err
}

func (buf *Buffer) AppendByte(b byte) error {
	return buf.AppendBytes([]byte{b})
}
