package types

import (
	"bytes"
	"errors"
	"math"
	"net"
)

type Property struct {
	IPAddress    net.IP
	ObjectId     *ObjectId
	ID           PropertyId
	Index        uint32
	HasIndex     bool
	Values       []*PropertyValue
	EncodedValue []byte
	Priority     uint8
	Length       int
}

func (p *Property) ReadValue(dest interface{}) {
	dest = p.Values
}

func (p *Property) ReadValuesAsObjects() []*Object {
	dest := make([]*Object, 0, len(p.Values))
	for _, val := range p.Values {
		v := val.ReadAsObject()
		dest = append(dest, &v)
	}
	return dest
}

func (p *Property) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	tag := GetTag()
	defer tag.Release()

	if p.ObjectId != nil {
		tag.TagNumber = 0
		tag.LenValue = 4
		buff.Write(tag.EncodeContextTag())
		objectID, err := p.ObjectId.MarshalBinary()
		if err != nil {
			return nil, err
		}
		buff.Write(objectID)
	}

	propertyID := EncodeVarUint(p.ID)
	tag.TagNumber = 1
	tag.LenValue = len(propertyID)
	buff.Write(tag.EncodeContextTag())
	buff.Write(propertyID)

	// Zero was historically the default and meant that no index was encoded.
	// HasIndex makes the valid array index zero expressible without changing
	// existing callers; non-zero indexes continue to work without the flag.
	if p.HasIndex || (p.Index != 0 && p.Index != BacnetArrayAll) {
		index := EncodeVarUint(p.Index)
		tag.TagNumber = 2
		tag.LenValue = len(index)
		buff.Write(tag.EncodeContextTag())
		buff.Write(index)
	}

	if len(p.Values) > 0 || p.EncodedValue != nil {
		tag.TagNumber = 3
		buff.Write(tag.EncodeOpeningTag())
		if p.EncodedValue != nil {
			buff.Write(p.EncodedValue)
		} else {
			for _, value := range p.Values {
				if value == nil {
					return nil, errors.New("property contains a nil value")
				}
				encoded, err := value.MarshalBinary()
				if err != nil {
					return nil, err
				}
				buff.Write(encoded)
			}
		}
		buff.Write(tag.EncodeClosingTag())
	}

	return buff.Bytes(), nil
}

func (p *Property) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	firstTag := &Tag{}
	if firstTag.DecodeTag(b) == 0 {
		return errors.New("malformed property tag")
	}
	full := firstTag.IsContext(0) && !firstTag.Opening && !firstTag.Closing && firstTag.LenValue == 4
	objectID := p.ObjectId
	ipAddress := p.IPAddress
	*p = Property{}
	p.IPAddress = ipAddress
	if !full {
		p.ObjectId = objectID
	}
	tagStart := uint8(0)
	offset := 0
	decode := func() (*Tag, int, error) {
		if offset >= len(b) {
			return nil, 0, errors.New("unexpected end of property data")
		}
		tag := &Tag{}
		headerLength := tag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return nil, 0, errors.New("malformed property tag")
		}
		return tag, headerLength, nil
	}
	readValue := func(tag *Tag, headerLength int) ([]byte, error) {
		if tag.LenValue < 0 || len(b)-offset-headerLength < tag.LenValue {
			return nil, errors.New("property value is truncated")
		}
		start := offset + headerLength
		offset = start + tag.LenValue
		return b[start:offset], nil
	}

	if full {
		tag, headerLength, err := decode()
		if err != nil {
			return err
		}
		if !tag.IsContext(0) || tag.Opening || tag.Closing || tag.LenValue != 4 {
			return errors.New("unexpected object identifier tag")
		}
		value, err := readValue(tag, headerLength)
		if err != nil {
			return err
		}
		p.ObjectId = &ObjectId{}
		if err := p.ObjectId.UnmarshalBinary(value); err != nil {
			return err
		}
		tagStart = 1
	}

	tag, headerLength, err := decode()
	if err != nil {
		return err
	}
	if !tag.IsContext(tagStart) || tag.Opening || tag.Closing || tag.LenValue < 1 || tag.LenValue > 4 {
		return errors.New("unexpected property identifier tag")
	}
	value, err := readValue(tag, headerLength)
	if err != nil {
		return err
	}
	p.ID = DecodeVarUint(value)
	p.Index = math.MaxUint32
	p.HasIndex = false

	if offset < len(b) {
		tag, headerLength, err = decode()
		if err != nil {
			return err
		}
		if tag.IsContext(tagStart+1) && !tag.Opening && !tag.Closing {
			if tag.LenValue < 1 || tag.LenValue > 4 {
				return errors.New("invalid property array index length")
			}
			value, err = readValue(tag, headerLength)
			if err != nil {
				return err
			}
			p.Index = DecodeVarUint(value)
			p.HasIndex = true
		}
	}

	// A ReadProperty request ends after its optional array index. A response or
	// WriteProperty request continues with the constructed property value.
	if offset == len(b) {
		p.Values = nil
		p.EncodedValue = nil
		p.Length = offset
		return nil
	}

	tag, headerLength, err = decode()
	if err != nil {
		return err
	}
	if !tag.IsContext(tagStart+2) || !tag.Opening {
		return errors.New("expected opening property value tag")
	}
	offset += headerLength
	contentStart := offset
	contentEnd, valueEnd, err := propertyValueBounds(b, contentStart, tagStart+2)
	if err != nil {
		return err
	}

	values := make([]*PropertyValue, 0, 1)
	for offset < contentEnd {
		tag, headerLength, err = decode()
		if err != nil {
			return err
		}
		if tag.Context || tag.Opening || tag.Closing {
			p.Values = nil
			p.EncodedValue = append([]byte(nil), b[contentStart:contentEnd]...)
			p.Length = valueEnd
			return nil
		}
		payloadLength := tag.LenValue
		if tag.TagNumber == TagNull || tag.TagNumber == TagBoolean {
			payloadLength = 0
		}
		if payloadLength < 0 || len(b)-offset-headerLength < payloadLength {
			return errors.New("property value is truncated")
		}
		encodedLength := headerLength + payloadLength
		propertyValue := &PropertyValue{}
		if err := propertyValue.UnmarshalBinary(b[offset : offset+encodedLength]); err != nil {
			return err
		}
		values = append(values, propertyValue)
		offset += encodedLength
	}

	p.Values = values
	p.EncodedValue = nil
	p.Length = valueEnd
	return nil
}

func propertyValueBounds(b []byte, offset int, outerTag uint8) (int, int, error) {
	stack := []uint8{outerTag}
	for offset < len(b) {
		start := offset
		tag := &Tag{}
		headerLength := tag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return 0, 0, errors.New("malformed tag in property value")
		}
		offset += headerLength
		switch {
		case tag.Opening:
			stack = append(stack, tag.TagNumber)
		case tag.Closing:
			if len(stack) == 0 || stack[len(stack)-1] != tag.TagNumber {
				return 0, 0, errors.New("mismatched closing tag in property value")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return start, offset, nil
			}
		default:
			payloadLength := tag.LenValue
			if !tag.Context && (tag.TagNumber == TagNull || tag.TagNumber == TagBoolean) {
				payloadLength = 0
			}
			if payloadLength < 0 || len(b)-offset < payloadLength {
				return 0, 0, errors.New("property value is truncated")
			}
			offset += payloadLength
		}
	}
	return 0, 0, errors.New("property value is not closed")
}
