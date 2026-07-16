package pdu

import (
	"errors"
	"fmt"

	"github.com/zyra/gobac/bacnet/types"
)

type CovNotification struct {
	ProcessIdentifier   uint8
	ProcessIdentifier32 uint32
	DeviceObjectId      *types.ObjectId
	ObjectId            *types.ObjectId
	TimeRemaining       uint32
	Properties          []*types.Property
	Priority            uint8
}

func (n *CovNotification) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}
	offset := 0
	readContextValue := func(number uint8, minLength, maxLength int) ([]byte, error) {
		if offset >= len(b) {
			return nil, errors.New("COV notification is truncated")
		}
		tag := &types.Tag{}
		headerLength := tag.DecodeTag(b[offset:])
		if headerLength == 0 || !tag.IsContext(number) || tag.Opening || tag.Closing {
			return nil, fmt.Errorf("expected context tag %d", number)
		}
		if tag.LenValue < minLength || tag.LenValue > maxLength || len(b)-offset-headerLength < tag.LenValue {
			return nil, errors.New("COV notification value is truncated")
		}
		offset += headerLength
		value := b[offset : offset+tag.LenValue]
		offset += tag.LenValue
		return value, nil
	}

	processID, err := readContextValue(0, 1, 4)
	if err != nil {
		return err
	}
	n.ProcessIdentifier32 = types.DecodeVarUint(processID)
	n.ProcessIdentifier = uint8(n.ProcessIdentifier32)

	deviceObjectID, err := readContextValue(1, 4, 4)
	if err != nil {
		return err
	}
	n.DeviceObjectId = &types.ObjectId{}
	if err := n.DeviceObjectId.UnmarshalBinary(deviceObjectID); err != nil {
		return err
	}

	monitoredObjectID, err := readContextValue(2, 4, 4)
	if err != nil {
		return err
	}
	n.ObjectId = &types.ObjectId{}
	if err := n.ObjectId.UnmarshalBinary(monitoredObjectID); err != nil {
		return err
	}

	timeRemaining, err := readContextValue(3, 1, 4)
	if err != nil {
		return err
	}
	n.TimeRemaining = types.DecodeVarUint(timeRemaining)

	if offset >= len(b) {
		return errors.New("COV notification has no list-of-values")
	}
	listTag := &types.Tag{}
	headerLength := listTag.DecodeTag(b[offset:])
	if headerLength == 0 || !listTag.IsContext(4) || !listTag.Opening {
		return errors.New("expected opening list-of-values tag")
	}
	offset += headerLength

	properties := make([]*types.Property, 0, 2)
	for {
		if offset >= len(b) {
			return errors.New("COV list-of-values is not closed")
		}
		tag := &types.Tag{}
		headerLength = tag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return errors.New("malformed COV property tag")
		}
		if tag.IsContext(4) && tag.Closing {
			offset += headerLength
			break
		}

		property := &types.Property{ObjectId: n.ObjectId}
		if err := property.UnmarshalBinary(b[offset:]); err != nil {
			return err
		}
		if property.Length <= 0 {
			return errors.New("COV property consumed no data")
		}
		offset += property.Length
		properties = append(properties, property)

		if offset < len(b) {
			tag = &types.Tag{}
			headerLength = tag.DecodeTag(b[offset:])
			if headerLength == 0 {
				return errors.New("malformed COV priority tag")
			}
			if tag.IsContext(3) && !tag.Opening && !tag.Closing {
				if tag.LenValue < 1 || tag.LenValue > 4 || len(b)-offset-headerLength < tag.LenValue {
					return errors.New("COV priority is truncated")
				}
				offset += headerLength
				priority := types.DecodeVarUint(b[offset : offset+tag.LenValue])
				if priority < 1 || priority > 16 {
					return errors.New("COV priority must be between 1 and 16")
				}
				property.Priority = uint8(priority)
				n.Priority = property.Priority
				offset += tag.LenValue
			}
		}
	}

	if offset != len(b) {
		return errors.New("unexpected trailing COV notification data")
	}
	n.Properties = properties
	return nil
}
