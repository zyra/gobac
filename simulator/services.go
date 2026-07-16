package simulator

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/zyra/gobac/v2/bacnet/types"
)

type PropertyReference struct {
	ID         uint32
	ArrayIndex *uint32
}

type ReadAccessSpecification struct {
	Object     ObjectID
	Properties []PropertyReference
}

type PropertyResult struct {
	Reference  PropertyReference
	Values     []Value
	ErrorClass types.ErrorClass
	ErrorCode  types.ErrorCode
}

type ReadAccessResult struct {
	Object  ObjectID
	Results []PropertyResult
}

type SubscribeCOVRequest struct {
	ProcessIdentifier uint32
	Object            ObjectID
	Confirmed         bool
	Lifetime          uint32
	Cancel            bool
}

var errWritePriorityOutOfRange = errors.New("WriteProperty priority is out of range")

func decodeReadProperty(data []byte) (ObjectID, PropertyReference, error) {
	property := &types.Property{}
	if err := property.UnmarshalBinary(data); err != nil {
		return ObjectID{}, PropertyReference{}, err
	}
	if property.ObjectId == nil || property.Length != len(data) {
		return ObjectID{}, PropertyReference{}, errors.New("invalid ReadProperty request")
	}
	reference := PropertyReference{ID: uint32(property.ID)}
	if property.HasIndex {
		index := property.Index
		reference.ArrayIndex = &index
	}
	return fromBACnetObjectID(property.ObjectId), reference, nil
}

func encodeReadPropertyResult(object ObjectID, reference PropertyReference, values []Value) ([]byte, error) {
	objectID, err := toBACnetObjectID(object)
	if err != nil {
		return nil, err
	}
	property := &types.Property{
		ObjectId: objectID,
		ID:       types.PropertyId(reference.ID),
		Values:   make([]*types.PropertyValue, 0, len(values)),
	}
	if reference.ArrayIndex != nil {
		property.HasIndex = true
		property.Index = *reference.ArrayIndex
	}
	for _, value := range values {
		converted, err := toPropertyValue(value)
		if err != nil {
			return nil, err
		}
		property.Values = append(property.Values, converted)
	}
	return property.MarshalBinary()
}

func decodeWriteProperty(data []byte) (ObjectID, PropertyReference, []Value, uint8, error) {
	property := &types.Property{}
	if err := property.UnmarshalBinary(data); err != nil {
		return ObjectID{}, PropertyReference{}, nil, 0, err
	}
	if property.ObjectId == nil || len(property.Values) == 0 || property.Length > len(data) {
		return ObjectID{}, PropertyReference{}, nil, 0, errors.New("invalid WriteProperty request")
	}
	reference := PropertyReference{ID: uint32(property.ID)}
	if property.HasIndex {
		index := property.Index
		reference.ArrayIndex = &index
	}
	values := make([]Value, 0, len(property.Values))
	for _, value := range property.Values {
		converted, err := fromPropertyValue(value)
		if err != nil {
			return ObjectID{}, PropertyReference{}, nil, 0, err
		}
		values = append(values, converted)
	}

	priority := uint8(0)
	if property.Length < len(data) {
		value, consumed, err := decodeContextUnsigned(data[property.Length:], 4)
		if err != nil || property.Length+consumed != len(data) {
			return ObjectID{}, PropertyReference{}, nil, 0, errors.New("invalid WriteProperty priority")
		}
		if value < 1 || value > PrioritySlots {
			return ObjectID{}, PropertyReference{}, nil, 0, errWritePriorityOutOfRange
		}
		priority = uint8(value)
	}
	return fromBACnetObjectID(property.ObjectId), reference, values, priority, nil
}

func decodeWhoIs(data []byte) (*uint32, *uint32, error) {
	if len(data) == 0 {
		return nil, nil, nil
	}
	low, consumed, err := decodeContextUnsigned(data, 0)
	if err != nil || low > MaxObjectInstance {
		return nil, nil, errors.New("invalid Who-Is low limit")
	}
	high, second, err := decodeContextUnsigned(data[consumed:], 1)
	if err != nil || high > MaxObjectInstance || consumed+second != len(data) || low > high {
		return nil, nil, errors.New("invalid Who-Is high limit")
	}
	return &low, &high, nil
}

func encodeIAm(device *Device) ([]byte, error) {
	if device == nil {
		return nil, errors.New("device is required")
	}
	values := []Value{
		{Tag: types.TagObjectId, Value: ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}},
		{Tag: types.TagUnsigned, Value: uint32(types.MaxApdu)},
		{Tag: types.TagEnumerated, Value: uint32(3)},
		{Tag: types.TagUnsigned, Value: uint32(device.VendorID)},
	}
	var out bytes.Buffer
	for _, value := range values {
		converted, err := toPropertyValue(value)
		if err != nil {
			return nil, err
		}
		encoded, err := converted.MarshalBinary()
		if err != nil {
			return nil, err
		}
		out.Write(encoded)
	}
	return out.Bytes(), nil
}

func decodeReadPropertyMultiple(data []byte) ([]ReadAccessSpecification, error) {
	offset := 0
	result := make([]ReadAccessSpecification, 0, 1)
	for offset < len(data) {
		objectBytes, consumed, err := decodeContext(data[offset:], 0)
		if err != nil || len(objectBytes) != 4 {
			return nil, errors.New("invalid ReadPropertyMultiple object identifier")
		}
		offset += consumed
		objectID := &types.ObjectId{}
		if err := objectID.UnmarshalBinary(objectBytes); err != nil {
			return nil, err
		}
		opening, err := decodeConstructedTag(data[offset:], 1, true)
		if err != nil {
			return nil, err
		}
		offset += opening

		specification := ReadAccessSpecification{Object: fromBACnetObjectID(objectID)}
		for {
			closing, err := isConstructedTag(data[offset:], 1, false)
			if err != nil {
				return nil, err
			}
			if closing > 0 {
				offset += closing
				break
			}
			propertyID, consumed, err := decodeContextUnsigned(data[offset:], 0)
			if err != nil {
				return nil, errors.New("invalid ReadPropertyMultiple property identifier")
			}
			offset += consumed
			reference := PropertyReference{ID: propertyID}
			if offset < len(data) {
				if index, optional, ok := tryDecodeContextUnsigned(data[offset:], 1); ok {
					offset += optional
					reference.ArrayIndex = &index
				}
			}
			specification.Properties = append(specification.Properties, reference)
		}
		if len(specification.Properties) == 0 {
			return nil, errors.New("ReadPropertyMultiple property list is empty")
		}
		result = append(result, specification)
	}
	if len(result) == 0 {
		return nil, errors.New("ReadPropertyMultiple request is empty")
	}
	return result, nil
}

func encodeReadPropertyMultipleResult(results []ReadAccessResult) ([]byte, error) {
	var out bytes.Buffer
	for _, access := range results {
		objectID, err := toBACnetObjectID(access.Object)
		if err != nil {
			return nil, err
		}
		encodedObject, err := objectID.MarshalBinary()
		if err != nil {
			return nil, err
		}
		writeContext(&out, 0, encodedObject)
		writeConstructedTag(&out, 1, true)
		for _, result := range access.Results {
			writeContext(&out, 2, types.EncodeVarUint(result.Reference.ID))
			if result.Reference.ArrayIndex != nil {
				writeContext(&out, 3, types.EncodeVarUint(*result.Reference.ArrayIndex))
			}
			if result.ErrorCode != 0 {
				writeConstructedTag(&out, 5, true)
				for _, value := range []uint32{uint32(result.ErrorClass), uint32(result.ErrorCode)} {
					propertyValue := &types.PropertyValue{Type: types.TagEnumerated, Value: value}
					encoded, err := propertyValue.MarshalBinary()
					if err != nil {
						return nil, err
					}
					out.Write(encoded)
				}
				writeConstructedTag(&out, 5, false)
				continue
			}
			writeConstructedTag(&out, 4, true)
			for _, value := range result.Values {
				converted, err := toPropertyValue(value)
				if err != nil {
					return nil, err
				}
				encoded, err := converted.MarshalBinary()
				if err != nil {
					return nil, err
				}
				out.Write(encoded)
			}
			writeConstructedTag(&out, 4, false)
		}
		writeConstructedTag(&out, 1, false)
	}
	return out.Bytes(), nil
}

func decodeSubscribeCOV(data []byte) (SubscribeCOVRequest, error) {
	var request SubscribeCOVRequest
	processIdentifier, consumed, err := decodeContextUnsigned(data, 0)
	if err != nil {
		return request, errors.New("invalid SubscribeCOV process identifier")
	}
	request.ProcessIdentifier = processIdentifier
	objectBytes, second, err := decodeContext(data[consumed:], 1)
	if err != nil || len(objectBytes) != 4 {
		return request, errors.New("invalid SubscribeCOV object identifier")
	}
	consumed += second
	objectID := &types.ObjectId{}
	if err := objectID.UnmarshalBinary(objectBytes); err != nil {
		return request, err
	}
	request.Object = fromBACnetObjectID(objectID)
	if consumed == len(data) {
		request.Cancel = true
		return request, nil
	}
	if tag := (&types.Tag{}); tag.DecodeTag(data[consumed:]) > 0 && tag.IsContext(2) && !tag.Opening && !tag.Closing {
		confirmed, second, err := decodeContext(data[consumed:], 2)
		if err != nil || len(confirmed) != 1 || (confirmed[0] != 0 && confirmed[0] != 1) {
			return request, errors.New("invalid SubscribeCOV confirmed-notification flag")
		}
		request.Confirmed = confirmed[0] != 0
		consumed += second
	}
	if consumed < len(data) {
		lifetime, second, err := decodeContextUnsigned(data[consumed:], 3)
		if err != nil {
			return request, errors.New("invalid SubscribeCOV lifetime")
		}
		request.Lifetime = lifetime
		consumed += second
	}
	if consumed != len(data) {
		return request, errors.New("unexpected trailing SubscribeCOV data")
	}
	return request, nil
}

func encodeCOVNotification(processID uint32, deviceID uint32, object ObjectID, timeRemaining uint32, properties []PropertyResult) ([]byte, error) {
	deviceObject, err := toBACnetObjectID(ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: deviceID})
	if err != nil {
		return nil, err
	}
	monitoredObject, err := toBACnetObjectID(object)
	if err != nil {
		return nil, err
	}
	deviceBytes, err := deviceObject.MarshalBinary()
	if err != nil {
		return nil, err
	}
	objectBytes, err := monitoredObject.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	writeContext(&out, 0, types.EncodeVarUint(processID))
	writeContext(&out, 1, deviceBytes)
	writeContext(&out, 2, objectBytes)
	writeContext(&out, 3, types.EncodeVarUint(timeRemaining))
	writeConstructedTag(&out, 4, true)
	for _, property := range properties {
		writeContext(&out, 0, types.EncodeVarUint(property.Reference.ID))
		if property.Reference.ArrayIndex != nil {
			writeContext(&out, 1, types.EncodeVarUint(*property.Reference.ArrayIndex))
		}
		writeConstructedTag(&out, 2, true)
		for _, value := range property.Values {
			converted, err := toPropertyValue(value)
			if err != nil {
				return nil, err
			}
			encoded, err := converted.MarshalBinary()
			if err != nil {
				return nil, err
			}
			out.Write(encoded)
		}
		writeConstructedTag(&out, 2, false)
	}
	writeConstructedTag(&out, 4, false)
	return out.Bytes(), nil
}

func toPropertyValue(value Value) (*types.PropertyValue, error) {
	converted := value.Value
	switch value.Tag {
	case types.TagCharacterString:
		switch typed := converted.(type) {
		case string:
			converted = types.CharacterString{Encoding: types.EncodingUtf8, Value: typed}
		}
	case types.TagObjectId:
		switch typed := converted.(type) {
		case ObjectID:
			objectID, err := toBACnetObjectID(typed)
			if err != nil {
				return nil, err
			}
			converted = objectID
		}
	}
	return &types.PropertyValue{Type: types.DataType(value.Tag), Value: converted}, nil
}

func fromPropertyValue(value *types.PropertyValue) (Value, error) {
	if value == nil {
		return Value{}, errors.New("nil property value")
	}
	converted := value.Value
	switch typed := value.Value.(type) {
	case types.CharacterString:
		converted = typed.Value
	case types.ObjectId:
		converted = fromBACnetObjectID(&typed)
	}
	return Value{Tag: uint8(value.Type), Value: converted}, nil
}

func toBACnetObjectID(value ObjectID) (*types.ObjectId, error) {
	objectID := &types.ObjectId{Type: types.ObjectType(value.Type)}
	if err := objectID.SetInstanceNumber(value.Instance); err != nil {
		return nil, err
	}
	return objectID, nil
}

func fromBACnetObjectID(value *types.ObjectId) ObjectID {
	return ObjectID{Type: uint16(value.Type), Instance: value.InstanceNumber()}
}

func decodeContext(data []byte, number uint8) ([]byte, int, error) {
	if len(data) == 0 {
		return nil, 0, errors.New("context value is truncated")
	}
	tag := &types.Tag{}
	headerLength := tag.DecodeTag(data)
	if headerLength == 0 || !tag.IsContext(number) || tag.Opening || tag.Closing || tag.LenValue < 0 || len(data)-headerLength < tag.LenValue {
		return nil, 0, fmt.Errorf("expected context tag %d", number)
	}
	end := headerLength + tag.LenValue
	return data[headerLength:end], end, nil
}

func decodeContextUnsigned(data []byte, number uint8) (uint32, int, error) {
	value, consumed, err := decodeContext(data, number)
	if err != nil || len(value) < 1 || len(value) > 4 {
		return 0, 0, fmt.Errorf("invalid unsigned context tag %d", number)
	}
	return types.DecodeVarUint(value), consumed, nil
}

func tryDecodeContextUnsigned(data []byte, number uint8) (uint32, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}
	tag := &types.Tag{}
	if tag.DecodeTag(data) == 0 || !tag.IsContext(number) || tag.Opening || tag.Closing {
		return 0, 0, false
	}
	value, consumed, err := decodeContextUnsigned(data, number)
	return value, consumed, err == nil
}

func decodeConstructedTag(data []byte, number uint8, opening bool) (int, error) {
	consumed, err := isConstructedTag(data, number, opening)
	if err != nil || consumed == 0 {
		return 0, fmt.Errorf("expected constructed tag %d", number)
	}
	return consumed, nil
}

func isConstructedTag(data []byte, number uint8, opening bool) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("constructed tag is truncated")
	}
	tag := &types.Tag{}
	headerLength := tag.DecodeTag(data)
	if headerLength == 0 {
		return 0, errors.New("constructed tag is malformed")
	}
	if !tag.IsContext(number) || tag.Opening != opening || tag.Closing == opening {
		return 0, nil
	}
	return headerLength, nil
}

func writeContext(out *bytes.Buffer, number uint8, value []byte) {
	tag := &types.Tag{TagNumber: number, LenValue: len(value)}
	out.Write(tag.EncodeContextTag())
	out.Write(value)
}

func writeConstructedTag(out *bytes.Buffer, number uint8, opening bool) {
	tag := &types.Tag{TagNumber: number}
	if opening {
		out.Write(tag.EncodeOpeningTag())
	} else {
		out.Write(tag.EncodeClosingTag())
	}
}
