package types

import "errors"

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
	return append([]byte{cs.Encoding}, []byte(cs.Value)...), nil
}

func (cs *CharacterString) UnmarshalBinary(b []byte) error {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	cs.Encoding = b[0]
	cs.Value = string(b[1:])
	return nil
}
