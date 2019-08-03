package encoding

func (buf *Buffer) EncodeCharacterString(val string) error {
	// Append UTF encoding
	if err := buf.AppendByte(0); err != nil {
		return err
	}

	// Append string
	return buf.AppendBytes([]byte(val))
}

func (buf *Buffer) DecodeCharacterString(lenValue uint32) string {
	_ = buf.NextOne() // encoding
	return string(buf.Next(int(lenValue)))
}
