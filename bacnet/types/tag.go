package types

import "sync"

var tagPool = sync.Pool{
	New: func() interface{} {
		return &Tag{}
	},
}

type Tag struct {
	TagNumber uint8
	LenValue  int
	Context   bool
	Extended  bool
	Opening   bool
	Closing   bool
}

func GetTag() *Tag {
	return tagPool.Get().(*Tag)
}

func (ct *Tag) Release() {
	ct.Reset()
	tagPool.Put(ct)
}

func (ct *Tag) Is(n uint8) bool {
	return ct.TagNumber == n
}

func (ct *Tag) IsContext(n uint8) bool {
	return ct.Context && ct.Is(n)
}

func (ct *Tag) Isnt(n uint8) bool {
	return ct.TagNumber != n
}

func (ct *Tag) Reset() {
	*ct = Tag{}
}

func (ct *Tag) encodeTag(first byte, constructed bool) []byte {
	b := make([]byte, 1, 7)
	b[0] = first
	if ct.TagNumber <= 14 {
		b[0] |= byte(ct.TagNumber) << 4
	} else {
		b[0] |= 0xf0
		b = append(b, byte(ct.TagNumber))
	}

	if constructed {
		return b
	}
	if ct.LenValue < 0 || uint64(ct.LenValue) > uint64(^uint32(0)) {
		return nil
	}
	if ct.LenValue <= 4 {
		b[0] |= byte(ct.LenValue)
		return b
	}

	b[0] |= 5
	switch {
	case ct.LenValue <= 253:
		b = append(b, byte(ct.LenValue))
	case ct.LenValue <= 65535:
		b = append(b, 254, byte(ct.LenValue>>8), byte(ct.LenValue))
	default:
		value := uint32(ct.LenValue)
		b = append(b, 255, byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	return b
}

func (ct *Tag) EncodeOpeningTag() []byte {
	return ct.encodeTag(0x08|6, true)
}

func (ct *Tag) EncodeClosingTag() []byte {
	return ct.encodeTag(0x08|7, true)
}

func (ct *Tag) EncodeContextTag() []byte {
	return ct.encodeTag(0x08, false)
}

func (ct *Tag) EncodeTag() []byte {
	return ct.encodeTag(0, false)
}

// DecodeTag decodes a BACnet tag header and returns its encoded length. A
// zero return value indicates a truncated or malformed header.
func (ct *Tag) DecodeTag(b []byte) int {
	ct.Reset()
	if len(b) == 0 {
		return 0
	}

	first := b[0]
	ct.Context = first&BIT3 != 0
	index := 1
	if first&0xf0 == 0xf0 {
		if len(b) <= index {
			ct.Reset()
			return 0
		}
		ct.TagNumber = b[index]
		ct.Extended = true
		index++
	} else {
		ct.TagNumber = first >> 4
	}

	switch first & 0x07 {
	case 5:
		ct.Extended = true
		if len(b) <= index {
			ct.Reset()
			return 0
		}
		marker := b[index]
		index++
		switch marker {
		case 255:
			if len(b)-index < 4 {
				ct.Reset()
				return 0
			}
			ct.LenValue = int(uint32(b[index])<<24 | uint32(b[index+1])<<16 | uint32(b[index+2])<<8 | uint32(b[index+3]))
			index += 4
		case 254:
			if len(b)-index < 2 {
				ct.Reset()
				return 0
			}
			ct.LenValue = int(uint16(b[index])<<8 | uint16(b[index+1]))
			index += 2
		default:
			ct.LenValue = int(marker)
		}
	case 6:
		ct.Opening = true
	case 7:
		ct.Closing = true
	default:
		ct.LenValue = int(first & 0x07)
	}

	return index
}
