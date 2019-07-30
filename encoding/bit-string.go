package encoding

import "math/bits"

func (buf *Buffer) EncodeBitString(val []byte) error {
	for _, b := range val {
		if err := buf.AppendByte(bits.Reverse8(b)); err != nil {
			return err
		}
	}

	return nil
}

func (buf *Buffer) DecodeBitString(lenValue uint32) (out []byte) {
	_ = buf.NextOne() // Unused bytes
	bytes := buf.Next(int(lenValue - 1))
	out = make([]byte, lenValue-1)

	for _, b := range bytes {
		out = append(out, bits.Reverse8(b))
	}

	return out
}
