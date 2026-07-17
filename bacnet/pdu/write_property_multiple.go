package pdu

import (
	"bytes"
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// PropertyWrite is one BACnetPropertyValue: the property (and optional array
// index) to write, the value(s) to write to it, and an optional write
// priority.
type PropertyWrite struct {
	ID          types.PropertyId
	Index       uint32 // valid only when HasIndex
	HasIndex    bool
	Values      []*types.PropertyValue
	Priority    uint8 // valid only when HasPriority; 1..16
	HasPriority bool
}

// WriteAccessSpec names one object and the property values to write to it, as
// carried in a WritePropertyMultiple-Request.
type WriteAccessSpec struct {
	ObjectId   *types.ObjectId
	Properties []PropertyWrite
}

// WritePropertyMultiplePdu is the WritePropertyMultiple-Request payload: a
// SEQUENCE OF WriteAccessSpecification.
type WritePropertyMultiplePdu struct {
	Specs []WriteAccessSpec
}

func (p *WritePropertyMultiplePdu) GetPduType() uint8 {
	return uint8(types.PduTypeConfirmedServiceRequest)
}

func (p *WritePropertyMultiplePdu) MarshalBinary() ([]byte, error) {
	if len(p.Specs) == 0 {
		return nil, errors.New("WritePropertyMultiple request requires at least one write access specification")
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

func (p *WritePropertyMultiplePdu) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}
	offset := 0
	var specs []WriteAccessSpec
	for offset < len(b) {
		spec, consumed, err := decodeWriteAccessSpec(b[offset:])
		if err != nil {
			return err
		}
		specs = append(specs, *spec)
		offset += consumed
	}
	if len(specs) == 0 {
		return errors.New("WritePropertyMultiple request is empty")
	}
	p.Specs = specs
	return nil
}

func (s *WriteAccessSpec) MarshalBinary() ([]byte, error) {
	if s.ObjectId == nil {
		return nil, errors.New("write access specification requires an object identifier")
	}
	if len(s.Properties) == 0 {
		return nil, errors.New("write access specification requires at least one property")
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

	for i := range s.Properties {
		property := &s.Properties[i]
		if len(property.Values) == 0 {
			return nil, errors.New("property write requires at least one value")
		}

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

		tag.TagNumber = 2
		buff.Write(tag.EncodeOpeningTag())
		for _, value := range property.Values {
			if value == nil {
				return nil, errors.New("property write contains a nil value")
			}
			encoded, err := value.MarshalBinary()
			if err != nil {
				return nil, err
			}
			buff.Write(encoded)
		}
		buff.Write(tag.EncodeClosingTag())

		if property.HasPriority {
			if property.Priority < 1 || property.Priority > 16 {
				return nil, errors.New("write priority must be between 1 and 16")
			}
			priority := types.EncodeVarUint(uint32(property.Priority))
			tag.TagNumber = 3
			tag.LenValue = len(priority)
			buff.Write(tag.EncodeContextTag())
			buff.Write(priority)
		}
	}

	tag.TagNumber = 1
	buff.Write(tag.EncodeClosingTag())

	return buff.Bytes(), nil
}

func (s *WriteAccessSpec) UnmarshalBinary(b []byte) error {
	spec, consumed, err := decodeWriteAccessSpec(b)
	if err != nil {
		return err
	}
	if consumed != len(b) {
		return errors.New("unexpected trailing data after write access specification")
	}
	*s = *spec
	return nil
}

// decodeWriteAccessSpec decodes a single WriteAccessSpecification starting at
// b[0] and returns the number of bytes it consumed, so callers can decode a
// SEQUENCE OF them back to back.
func decodeWriteAccessSpec(b []byte) (*WriteAccessSpec, int, error) {
	if len(b) == 0 {
		return nil, 0, errors.New("write access specification is empty")
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
		return nil, 0, errors.New("write access specification has no property list")
	}
	listTag := &types.Tag{}
	headerLength = listTag.DecodeTag(b[offset:])
	if headerLength == 0 || !listTag.IsContext(1) || !listTag.Opening {
		return nil, 0, errors.New("expected an opening property list tag")
	}
	offset += headerLength

	spec := &WriteAccessSpec{ObjectId: objectID}
	for {
		if offset >= len(b) {
			return nil, 0, errors.New("write access specification property list is not closed")
		}
		propertyTag := &types.Tag{}
		headerLength = propertyTag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return nil, 0, errors.New("malformed property write tag")
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
		property := PropertyWrite{ID: types.DecodeVarUint(b[offset : offset+propertyTag.LenValue])}
		offset += propertyTag.LenValue

		if offset >= len(b) {
			return nil, 0, errors.New("property write is truncated")
		}
		nextTag := &types.Tag{}
		nextLength := nextTag.DecodeTag(b[offset:])
		if nextLength == 0 {
			return nil, 0, errors.New("malformed property write tag")
		}
		if nextTag.IsContext(1) && !nextTag.Opening && !nextTag.Closing {
			if nextTag.LenValue < 1 || nextTag.LenValue > 4 || len(b)-offset-nextLength < nextTag.LenValue {
				return nil, 0, errors.New("property array index is truncated")
			}
			offset += nextLength
			property.Index = types.DecodeVarUint(b[offset : offset+nextTag.LenValue])
			property.HasIndex = true
			offset += nextTag.LenValue

			if offset >= len(b) {
				return nil, 0, errors.New("property write is truncated")
			}
			nextTag = &types.Tag{}
			nextLength = nextTag.DecodeTag(b[offset:])
			if nextLength == 0 {
				return nil, 0, errors.New("malformed property write tag")
			}
		}

		if !nextTag.IsContext(2) || !nextTag.Opening {
			return nil, 0, errors.New("expected an opening property value tag")
		}
		offset += nextLength
		contentStart := offset
		contentEnd, valueEnd, err := propertyResultValueBounds(b, contentStart, 2)
		if err != nil {
			return nil, 0, err
		}
		values, err := decodePropertyResultValues(b[contentStart:contentEnd])
		if err != nil {
			return nil, 0, err
		}
		if len(values) == 0 {
			return nil, 0, errors.New("property write requires at least one value")
		}
		property.Values = values
		offset = valueEnd

		if offset < len(b) {
			priorityTag := &types.Tag{}
			peekLength := priorityTag.DecodeTag(b[offset:])
			if peekLength > 0 && priorityTag.IsContext(3) && !priorityTag.Opening && !priorityTag.Closing {
				if priorityTag.LenValue < 1 || priorityTag.LenValue > 4 || len(b)-offset-peekLength < priorityTag.LenValue {
					return nil, 0, errors.New("write priority is truncated")
				}
				offset += peekLength
				priority := types.DecodeVarUint(b[offset : offset+priorityTag.LenValue])
				if priority < 1 || priority > 16 {
					return nil, 0, errors.New("write priority must be between 1 and 16")
				}
				property.Priority = uint8(priority)
				property.HasPriority = true
				offset += priorityTag.LenValue
			}
		}

		spec.Properties = append(spec.Properties, property)
	}

	if len(spec.Properties) == 0 {
		return nil, 0, errors.New("write access specification requires at least one property")
	}

	return spec, offset, nil
}

// WritePropertyMultipleError carries the WritePropertyMultiple-Error
// production returned when a WritePropertyMultiple-Request could not be
// applied: the standard errorType, plus a reference to the property whose
// validation failed first.
type WritePropertyMultipleError struct {
	Class       types.ErrorClass
	Code        types.ErrorCode
	FirstFailed struct {
		ObjectId *types.ObjectId
		ID       types.PropertyId
		Index    uint32
		HasIndex bool
	}
}

func (e *WritePropertyMultipleError) MarshalBinary() ([]byte, error) {
	if e.FirstFailed.ObjectId == nil {
		return nil, errors.New("write property multiple error requires an object identifier")
	}

	buff := bytes.NewBuffer(nil)
	tag := types.GetTag()
	defer tag.Release()

	tag.TagNumber = 0
	buff.Write(tag.EncodeOpeningTag())
	class := &types.PropertyValue{Type: types.TagEnumerated, Value: uint32(e.Class)}
	encoded, err := class.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)
	code := &types.PropertyValue{Type: types.TagEnumerated, Value: uint32(e.Code)}
	encoded, err = code.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)
	buff.Write(tag.EncodeClosingTag())

	tag.TagNumber = 1
	buff.Write(tag.EncodeOpeningTag())

	tag.TagNumber = 0
	tag.LenValue = 4
	buff.Write(tag.EncodeContextTag())
	objectID, err := e.FirstFailed.ObjectId.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(objectID)

	id := types.EncodeVarUint(e.FirstFailed.ID)
	tag.TagNumber = 1
	tag.LenValue = len(id)
	buff.Write(tag.EncodeContextTag())
	buff.Write(id)

	if e.FirstFailed.HasIndex {
		index := types.EncodeVarUint(e.FirstFailed.Index)
		tag.TagNumber = 2
		tag.LenValue = len(index)
		buff.Write(tag.EncodeContextTag())
		buff.Write(index)
	}

	tag.TagNumber = 1
	buff.Write(tag.EncodeClosingTag())

	return buff.Bytes(), nil
}

func (e *WritePropertyMultipleError) UnmarshalBinary(b []byte) error {
	consumed, err := e.unmarshal(b)
	if err != nil {
		return err
	}
	if consumed != len(b) {
		return errors.New("unexpected trailing data after write property multiple error")
	}
	return nil
}

func (e *WritePropertyMultipleError) unmarshal(b []byte) (int, error) {
	tag := &types.Tag{}
	headerLength := tag.DecodeTag(b)
	if headerLength == 0 || !tag.IsContext(0) || !tag.Opening {
		return 0, errors.New("expected an opening error type tag")
	}
	offset := headerLength

	values := []*uint32{new(uint32), new(uint32)}
	for _, value := range values {
		if offset >= len(b) {
			return 0, errors.New("write property multiple error is truncated")
		}
		valueTag := &types.Tag{}
		valueHeader := valueTag.DecodeTag(b[offset:])
		if valueHeader == 0 || valueTag.Context || valueTag.TagNumber != types.TagEnumerated || valueTag.Opening || valueTag.Closing || valueTag.LenValue < 1 || valueTag.LenValue > 4 {
			return 0, errors.New("write property multiple error contains an invalid tag")
		}
		if len(b)-offset-valueHeader < valueTag.LenValue {
			return 0, errors.New("write property multiple error value is truncated")
		}
		offset += valueHeader
		*value = types.DecodeVarUint(b[offset : offset+valueTag.LenValue])
		offset += valueTag.LenValue
	}

	if offset >= len(b) {
		return 0, errors.New("write property multiple error is not closed")
	}
	closeTag := &types.Tag{}
	closeLength := closeTag.DecodeTag(b[offset:])
	if closeLength == 0 || !closeTag.IsContext(0) || !closeTag.Closing {
		return 0, errors.New("expected a closing error type tag")
	}
	offset += closeLength
	class := types.ErrorClass(*values[0])
	code := types.ErrorCode(*values[1])

	if offset >= len(b) {
		return 0, errors.New("write property multiple error is missing the failed reference")
	}
	listTag := &types.Tag{}
	listHeader := listTag.DecodeTag(b[offset:])
	if listHeader == 0 || !listTag.IsContext(1) || !listTag.Opening {
		return 0, errors.New("expected an opening failed-reference tag")
	}
	offset += listHeader

	if offset >= len(b) {
		return 0, errors.New("write property multiple error is missing an object identifier")
	}
	objTag := &types.Tag{}
	objHeader := objTag.DecodeTag(b[offset:])
	if objHeader == 0 || !objTag.IsContext(0) || objTag.Opening || objTag.Closing || objTag.LenValue != 4 {
		return 0, errors.New("expected an object identifier tag")
	}
	if len(b)-offset-objHeader < objTag.LenValue {
		return 0, errors.New("object identifier is truncated")
	}
	objectID := &types.ObjectId{}
	if err := objectID.UnmarshalBinary(b[offset+objHeader : offset+objHeader+objTag.LenValue]); err != nil {
		return 0, err
	}
	offset += objHeader + objTag.LenValue

	if offset >= len(b) {
		return 0, errors.New("write property multiple error is missing a property identifier")
	}
	propTag := &types.Tag{}
	propHeader := propTag.DecodeTag(b[offset:])
	if propHeader == 0 || !propTag.IsContext(1) || propTag.Opening || propTag.Closing || propTag.LenValue < 1 || propTag.LenValue > 4 {
		return 0, errors.New("expected a property identifier tag")
	}
	if len(b)-offset-propHeader < propTag.LenValue {
		return 0, errors.New("property identifier is truncated")
	}
	propertyID := types.DecodeVarUint(b[offset+propHeader : offset+propHeader+propTag.LenValue])
	offset += propHeader + propTag.LenValue

	index := uint32(0)
	hasIndex := false
	if offset < len(b) {
		peekTag := &types.Tag{}
		peekHeader := peekTag.DecodeTag(b[offset:])
		if peekHeader > 0 && peekTag.IsContext(2) && !peekTag.Opening && !peekTag.Closing {
			if peekTag.LenValue < 1 || peekTag.LenValue > 4 || len(b)-offset-peekHeader < peekTag.LenValue {
				return 0, errors.New("property array index is truncated")
			}
			index = types.DecodeVarUint(b[offset+peekHeader : offset+peekHeader+peekTag.LenValue])
			hasIndex = true
			offset += peekHeader + peekTag.LenValue
		}
	}

	if offset >= len(b) {
		return 0, errors.New("write property multiple error is not closed")
	}
	closeList := &types.Tag{}
	closeListLength := closeList.DecodeTag(b[offset:])
	if closeListLength == 0 || !closeList.IsContext(1) || !closeList.Closing {
		return 0, errors.New("expected a closing failed-reference tag")
	}
	offset += closeListLength

	e.Class = class
	e.Code = code
	e.FirstFailed.ObjectId = objectID
	e.FirstFailed.ID = propertyID
	e.FirstFailed.Index = index
	e.FirstFailed.HasIndex = hasIndex

	return offset, nil
}
