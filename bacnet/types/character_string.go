package types

const (
	EncodingAnsiX34  CharacterStringEncoding = 0
	EncodingUtf8                             = 0
	EncodingMsDbcs                           = 1
	EncodingJisc6226                         = 2
	EncodingUcs4                             = 3
	EncodingUcs2                             = 4
	EncodingIso8859                          = 5
	EncodingMax                              = 6
)

type CharacterStringEncoding = uint32

type CharacterString struct {
	Value    string
	Encoding uint8
}

func (cs *CharacterString) Length() int {
	return len(cs.Value) + 1
}

func (cs *CharacterString) MarshalBinary() ([]byte, error) {
	b := make([]byte, len(cs.Value)+1)
	b[0] = cs.Encoding
	copy(b[1:], []byte(cs.Value))
	return b, nil
}

func (cs *CharacterString) UnmarshalBinary(b []byte) error {
	cs.Encoding = b[0]
	cs.Value = string(b[1:])
	return nil
}
