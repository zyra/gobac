package encoding

func (buf *Buffer) EncodeContext(tag uint8, value uint32) (err error) {
	var tagLen uint32 = 0

	if value < 0x100 {
		tagLen = 1
	} else if value < 0x10000 {
		tagLen = 2
	} else if value < 0x1000000 {
		tagLen = 3
	} else {
		tagLen = 4
	}

	err = buf.EncodeTag(tag, true, tagLen)
	err = buf.EncodeUnsigned(value)

	return err
}
