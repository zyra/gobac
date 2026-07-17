package pdu

import (
	"bytes"
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// PropertyReference identifies a single property, and optionally an array
// index within it, requested from (or returned for) an object.
type PropertyReference struct {
	ID       types.PropertyId
	Index    uint32
	HasIndex bool
}

// ReadAccessSpec names one object and the properties requested from it, as
// carried in a ReadPropertyMultiple-Request.
type ReadAccessSpec struct {
	ObjectId   *types.ObjectId
	Properties []PropertyReference
}

// ReadPropertyMultiplePdu is the ReadPropertyMultiple-Request payload: a
// SEQUENCE OF ReadAccessSpecification.
type ReadPropertyMultiplePdu struct {
	Specs []ReadAccessSpec
}

// PropertyAccessError carries the errorClass/errorCode BACnet returns when a
// single property within a ReadPropertyMultiple result could not be read.
type PropertyAccessError struct {
	Class types.ErrorClass
	Code  types.ErrorCode
}

// PropertyResult is one property's outcome within a ReadAccessResult: either
// Values is populated, or Error is - never both.
type PropertyResult struct {
	ID       types.PropertyId
	Index    uint32
	HasIndex bool
	Values   []*types.PropertyValue
	Error    *PropertyAccessError
}

// ReadAccessResult is one object's results within a ReadPropertyMultiple
// acknowledgement.
type ReadAccessResult struct {
	ObjectId *types.ObjectId
	Results  []PropertyResult
}

// ReadPropertyMultipleAck is the ReadPropertyMultiple-ACK payload: a
// SEQUENCE OF ReadAccessResult.
type ReadPropertyMultipleAck struct {
	Results []ReadAccessResult
}

func (p *ReadPropertyMultiplePdu) GetPduType() uint8 {
	return uint8(types.PduTypeConfirmedServiceRequest)
}

func (p *ReadPropertyMultiplePdu) MarshalBinary() ([]byte, error) {
	if len(p.Specs) == 0 {
		return nil, errors.New("ReadPropertyMultiple request requires at least one read access specification")
	}
	buff := bytes.NewBuffer(nil)
	for i := range p.Specs {
		encoded, err := p.Specs[i].MarshalBinary()
		if err != nil {
			return nil, err
		}
		buff.Write(encoded)
	}
	return buff.Bytes(), nil
}

func (p *ReadPropertyMultiplePdu) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}
	offset := 0
	var specs []ReadAccessSpec
	for offset < len(b) {
		spec, consumed, err := decodeReadAccessSpec(b[offset:])
		if err != nil {
			return err
		}
		specs = append(specs, *spec)
		offset += consumed
	}
	if len(specs) == 0 {
		return errors.New("ReadPropertyMultiple request is empty")
	}
	p.Specs = specs
	return nil
}

func (s *ReadAccessSpec) MarshalBinary() ([]byte, error) {
	if s.ObjectId == nil {
		return nil, errors.New("read access specification requires an object identifier")
	}
	if len(s.Properties) == 0 {
		return nil, errors.New("read access specification requires at least one property")
	}

	buff := bytes.NewBuffer(nil)
	tag := types.GetTag()
	defer tag.Release()

	tag.TagNumber = 0
	tag.LenValue = 4
	buff.Write(tag.EncodeContextTag())
	objectID, err := s.ObjectId.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(objectID)

	tag.TagNumber = 1
	buff.Write(tag.EncodeOpeningTag())

	for _, property := range s.Properties {
		id := types.EncodeVarUint(property.ID)
		tag.TagNumber = 0
		tag.LenValue = len(id)
		buff.Write(tag.EncodeContextTag())
		buff.Write(id)

		if property.HasIndex {
			index := types.EncodeVarUint(property.Index)
			tag.TagNumber = 1
			tag.LenValue = len(index)
			buff.Write(tag.EncodeContextTag())
			buff.Write(index)
		}
	}

	tag.TagNumber = 1
	buff.Write(tag.EncodeClosingTag())

	return buff.Bytes(), nil
}

func (s *ReadAccessSpec) UnmarshalBinary(b []byte) error {
	spec, consumed, err := decodeReadAccessSpec(b)
	if err != nil {
		return err
	}
	if consumed != len(b) {
		return errors.New("unexpected trailing data after read access specification")
	}
	*s = *spec
	return nil
}

// decodeReadAccessSpec decodes a single ReadAccessSpecification starting at
// b[0] and returns the number of bytes it consumed, so callers can decode a
// SEQUENCE OF them back to back.
func decodeReadAccessSpec(b []byte) (*ReadAccessSpec, int, error) {
	if len(b) == 0 {
		return nil, 0, errors.New("read access specification is empty")
	}

	tag := &types.Tag{}
	headerLength := tag.DecodeTag(b)
	if headerLength == 0 || !tag.IsContext(0) || tag.Opening || tag.Closing || tag.LenValue != 4 {
		return nil, 0, errors.New("expected an object identifier tag")
	}
	if len(b)-headerLength < tag.LenValue {
		return nil, 0, errors.New("object identifier is truncated")
	}
	objectID := &types.ObjectId{}
	if err := objectID.UnmarshalBinary(b[headerLength : headerLength+tag.LenValue]); err != nil {
		return nil, 0, err
	}
	offset := headerLength + tag.LenValue

	if offset >= len(b) {
		return nil, 0, errors.New("read access specification has no property list")
	}
	listTag := &types.Tag{}
	headerLength = listTag.DecodeTag(b[offset:])
	if headerLength == 0 || !listTag.IsContext(1) || !listTag.Opening {
		return nil, 0, errors.New("expected an opening property list tag")
	}
	offset += headerLength

	spec := &ReadAccessSpec{ObjectId: objectID}
	for {
		if offset >= len(b) {
			return nil, 0, errors.New("read access specification property list is not closed")
		}
		propertyTag := &types.Tag{}
		headerLength = propertyTag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return nil, 0, errors.New("malformed property reference tag")
		}
		if propertyTag.IsContext(1) && propertyTag.Closing {
			offset += headerLength
			break
		}
		if !propertyTag.IsContext(0) || propertyTag.Opening || propertyTag.Closing || propertyTag.LenValue < 1 || propertyTag.LenValue > 4 {
			return nil, 0, errors.New("expected a property identifier tag")
		}
		if len(b)-offset-headerLength < propertyTag.LenValue {
			return nil, 0, errors.New("property identifier is truncated")
		}
		offset += headerLength
		reference := PropertyReference{ID: types.DecodeVarUint(b[offset : offset+propertyTag.LenValue])}
		offset += propertyTag.LenValue

		if offset < len(b) {
			indexTag := &types.Tag{}
			peekLength := indexTag.DecodeTag(b[offset:])
			if peekLength > 0 && indexTag.IsContext(1) && !indexTag.Opening && !indexTag.Closing {
				if indexTag.LenValue < 1 || indexTag.LenValue > 4 || len(b)-offset-peekLength < indexTag.LenValue {
					return nil, 0, errors.New("property array index is truncated")
				}
				offset += peekLength
				reference.Index = types.DecodeVarUint(b[offset : offset+indexTag.LenValue])
				reference.HasIndex = true
				offset += indexTag.LenValue
			}
		}

		spec.Properties = append(spec.Properties, reference)
	}

	if len(spec.Properties) == 0 {
		return nil, 0, errors.New("read access specification requires at least one property")
	}

	return spec, offset, nil
}

func (r *ReadAccessResult) MarshalBinary() ([]byte, error) {
	if r.ObjectId == nil {
		return nil, errors.New("read access result requires an object identifier")
	}
	if len(r.Results) == 0 {
		return nil, errors.New("read access result requires at least one property result")
	}

	buff := bytes.NewBuffer(nil)
	tag := types.GetTag()
	defer tag.Release()

	tag.TagNumber = 0
	tag.LenValue = 4
	buff.Write(tag.EncodeContextTag())
	objectID, err := r.ObjectId.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(objectID)

	tag.TagNumber = 1
	buff.Write(tag.EncodeOpeningTag())

	for i := range r.Results {
		property := &r.Results[i]
		if property.Values != nil && property.Error != nil {
			return nil, errors.New("property result cannot have both values and an error")
		}
		if property.Values == nil && property.Error == nil {
			return nil, errors.New("property result requires values or an error")
		}

		id := types.EncodeVarUint(property.ID)
		tag.TagNumber = 2
		tag.LenValue = len(id)
		buff.Write(tag.EncodeContextTag())
		buff.Write(id)

		if property.HasIndex {
			index := types.EncodeVarUint(property.Index)
			tag.TagNumber = 3
			tag.LenValue = len(index)
			buff.Write(tag.EncodeContextTag())
			buff.Write(index)
		}

		if property.Error != nil {
			tag.TagNumber = 5
			buff.Write(tag.EncodeOpeningTag())

			class := &types.PropertyValue{Type: types.TagEnumerated, Value: uint32(property.Error.Class)}
			encoded, err := class.MarshalBinary()
			if err != nil {
				return nil, err
			}
			buff.Write(encoded)

			code := &types.PropertyValue{Type: types.TagEnumerated, Value: uint32(property.Error.Code)}
			encoded, err = code.MarshalBinary()
			if err != nil {
				return nil, err
			}
			buff.Write(encoded)

			buff.Write(tag.EncodeClosingTag())
			continue
		}

		tag.TagNumber = 4
		buff.Write(tag.EncodeOpeningTag())
		for _, value := range property.Values {
			if value == nil {
				return nil, errors.New("property result contains a nil value")
			}
			encoded, err := value.MarshalBinary()
			if err != nil {
				return nil, err
			}
			buff.Write(encoded)
		}
		buff.Write(tag.EncodeClosingTag())
	}

	tag.TagNumber = 1
	buff.Write(tag.EncodeClosingTag())

	return buff.Bytes(), nil
}

func (r *ReadAccessResult) UnmarshalBinary(b []byte) error {
	result, consumed, err := decodeReadAccessResult(b)
	if err != nil {
		return err
	}
	if consumed != len(b) {
		return errors.New("unexpected trailing data after read access result")
	}
	*r = *result
	return nil
}

func (a *ReadPropertyMultipleAck) MarshalBinary() ([]byte, error) {
	if len(a.Results) == 0 {
		return nil, errors.New("ReadPropertyMultiple ack requires at least one result")
	}
	buff := bytes.NewBuffer(nil)
	for i := range a.Results {
		encoded, err := a.Results[i].MarshalBinary()
		if err != nil {
			return nil, err
		}
		buff.Write(encoded)
	}
	return buff.Bytes(), nil
}

func (a *ReadPropertyMultipleAck) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}
	offset := 0
	var results []ReadAccessResult
	for offset < len(b) {
		result, consumed, err := decodeReadAccessResult(b[offset:])
		if err != nil {
			return err
		}
		results = append(results, *result)
		offset += consumed
	}
	if len(results) == 0 {
		return errors.New("ReadPropertyMultiple ack is empty")
	}
	a.Results = results
	return nil
}

// decodeReadAccessResult decodes a single ReadAccessResult starting at b[0]
// and returns the number of bytes it consumed, so callers can decode a
// SEQUENCE OF them back to back.
func decodeReadAccessResult(b []byte) (*ReadAccessResult, int, error) {
	if len(b) == 0 {
		return nil, 0, errors.New("read access result is empty")
	}

	tag := &types.Tag{}
	headerLength := tag.DecodeTag(b)
	if headerLength == 0 || !tag.IsContext(0) || tag.Opening || tag.Closing || tag.LenValue != 4 {
		return nil, 0, errors.New("expected an object identifier tag")
	}
	if len(b)-headerLength < tag.LenValue {
		return nil, 0, errors.New("object identifier is truncated")
	}
	objectID := &types.ObjectId{}
	if err := objectID.UnmarshalBinary(b[headerLength : headerLength+tag.LenValue]); err != nil {
		return nil, 0, err
	}
	offset := headerLength + tag.LenValue

	if offset >= len(b) {
		return nil, 0, errors.New("read access result has no result list")
	}
	listTag := &types.Tag{}
	headerLength = listTag.DecodeTag(b[offset:])
	if headerLength == 0 || !listTag.IsContext(1) || !listTag.Opening {
		return nil, 0, errors.New("expected an opening result list tag")
	}
	offset += headerLength

	result := &ReadAccessResult{ObjectId: objectID}
	for {
		if offset >= len(b) {
			return nil, 0, errors.New("read access result list is not closed")
		}
		propertyTag := &types.Tag{}
		headerLength = propertyTag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return nil, 0, errors.New("malformed read access result tag")
		}
		if propertyTag.IsContext(1) && propertyTag.Closing {
			offset += headerLength
			break
		}
		if !propertyTag.IsContext(2) || propertyTag.Opening || propertyTag.Closing || propertyTag.LenValue < 1 || propertyTag.LenValue > 4 {
			return nil, 0, errors.New("expected a property identifier tag")
		}
		if len(b)-offset-headerLength < propertyTag.LenValue {
			return nil, 0, errors.New("property identifier is truncated")
		}
		offset += headerLength
		property := PropertyResult{ID: types.DecodeVarUint(b[offset : offset+propertyTag.LenValue])}
		offset += propertyTag.LenValue

		if offset >= len(b) {
			return nil, 0, errors.New("property result is truncated")
		}
		nextTag := &types.Tag{}
		nextLength := nextTag.DecodeTag(b[offset:])
		if nextLength == 0 {
			return nil, 0, errors.New("malformed property result tag")
		}
		if nextTag.IsContext(3) && !nextTag.Opening && !nextTag.Closing {
			if nextTag.LenValue < 1 || nextTag.LenValue > 4 || len(b)-offset-nextLength < nextTag.LenValue {
				return nil, 0, errors.New("property array index is truncated")
			}
			offset += nextLength
			property.Index = types.DecodeVarUint(b[offset : offset+nextTag.LenValue])
			property.HasIndex = true
			offset += nextTag.LenValue

			if offset >= len(b) {
				return nil, 0, errors.New("property result is truncated")
			}
			nextTag = &types.Tag{}
			nextLength = nextTag.DecodeTag(b[offset:])
			if nextLength == 0 {
				return nil, 0, errors.New("malformed property result tag")
			}
		}

		switch {
		case nextTag.IsContext(4) && nextTag.Opening:
			offset += nextLength
			contentStart := offset
			contentEnd, valueEnd, err := propertyResultValueBounds(b, contentStart, 4)
			if err != nil {
				return nil, 0, err
			}
			values, err := decodePropertyResultValues(b[contentStart:contentEnd])
			if err != nil {
				return nil, 0, err
			}
			property.Values = values
			offset = valueEnd

		case nextTag.IsContext(5) && nextTag.Opening:
			offset += nextLength
			values := []*uint32{new(uint32), new(uint32)}
			for _, value := range values {
				if offset >= len(b) {
					return nil, 0, errors.New("property error result is truncated")
				}
				errorTag := &types.Tag{}
				errorHeaderLength := errorTag.DecodeTag(b[offset:])
				if errorHeaderLength == 0 || errorTag.Context || errorTag.TagNumber != types.TagEnumerated || errorTag.Opening || errorTag.Closing || errorTag.LenValue < 1 || errorTag.LenValue > 4 {
					return nil, 0, errors.New("property error result contains an invalid tag")
				}
				if len(b)-offset-errorHeaderLength < errorTag.LenValue {
					return nil, 0, errors.New("property error result value is truncated")
				}
				offset += errorHeaderLength
				*value = types.DecodeVarUint(b[offset : offset+errorTag.LenValue])
				offset += errorTag.LenValue
			}
			if offset >= len(b) {
				return nil, 0, errors.New("property error result is not closed")
			}
			closeTag := &types.Tag{}
			closeLength := closeTag.DecodeTag(b[offset:])
			if closeLength == 0 || !closeTag.IsContext(5) || !closeTag.Closing {
				return nil, 0, errors.New("expected a closing property error tag")
			}
			offset += closeLength
			property.Error = &PropertyAccessError{
				Class: types.ErrorClass(*values[0]),
				Code:  types.ErrorCode(*values[1]),
			}

		default:
			return nil, 0, errors.New("expected a property value or error tag")
		}

		result.Results = append(result.Results, property)
	}

	if len(result.Results) == 0 {
		return nil, 0, errors.New("read access result requires at least one property result")
	}

	return result, offset, nil
}

// decodePropertyResultValues decodes a flat run of application-tagged
// property values, as found inside the [4] opening/closing tags of a
// ReadAccessResult property.
func decodePropertyResultValues(b []byte) ([]*types.PropertyValue, error) {
	values := make([]*types.PropertyValue, 0, 1)
	offset := 0
	for offset < len(b) {
		tag := &types.Tag{}
		headerLength := tag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return nil, errors.New("malformed property value tag")
		}
		if tag.Context || tag.Opening || tag.Closing {
			return nil, errors.New("unexpected constructed tag in property value list")
		}
		payloadLength := tag.LenValue
		if tag.TagNumber == types.TagNull || tag.TagNumber == types.TagBoolean {
			payloadLength = 0
		}
		if payloadLength < 0 || len(b)-offset-headerLength < payloadLength {
			return nil, errors.New("property value is truncated")
		}
		encodedLength := headerLength + payloadLength
		value := &types.PropertyValue{}
		if err := value.UnmarshalBinary(b[offset : offset+encodedLength]); err != nil {
			return nil, err
		}
		values = append(values, value)
		offset += encodedLength
	}
	return values, nil
}

// propertyResultValueBounds finds the byte range of the constructed [4]
// value list starting right after its opening tag at b[offset], tolerating
// arbitrarily nested constructed tags within it. It returns the content end
// (exclusive of the closing tag) and the offset immediately following the
// closing tag.
func propertyResultValueBounds(b []byte, offset int, outerTag uint8) (int, int, error) {
	stack := []uint8{outerTag}
	for offset < len(b) {
		start := offset
		tag := &types.Tag{}
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
			if !tag.Context && (tag.TagNumber == types.TagNull || tag.TagNumber == types.TagBoolean) {
				payloadLength = 0
			}
			if payloadLength < 0 || len(b)-offset < payloadLength {
				return 0, 0, errors.New("property value is truncated")
			}
			offset += payloadLength
		}
	}
	return 0, 0, errors.New("unterminated property value")
}
