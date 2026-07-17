package pdu

import (
	"bytes"
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// WhoHas is the Who-Has-Request payload (ASHRAE 135 §16.9): an optional
// device-instance range, followed by a choice of either an object
// identifier or an object name to search for. Exactly one of ObjectId /
// ObjectName must be set.
type WhoHas struct {
	LowLimit   uint32
	HighLimit  uint32
	HasRange   bool
	ObjectId   *types.ObjectId
	ObjectName string
}

func (p *WhoHas) MarshalBinary() ([]byte, error) {
	hasObjectId := p.ObjectId != nil
	hasObjectName := p.ObjectName != ""
	if hasObjectId == hasObjectName {
		return nil, errors.New("who-has requires exactly one of an object identifier or an object name")
	}

	buff := bytes.NewBuffer(nil)
	tag := types.GetTag()
	defer tag.Release()

	if p.HasRange {
		low := types.EncodeVarUint(p.LowLimit)
		tag.TagNumber = 0
		tag.LenValue = len(low)
		buff.Write(tag.EncodeContextTag())
		buff.Write(low)

		high := types.EncodeVarUint(p.HighLimit)
		tag.TagNumber = 1
		tag.LenValue = len(high)
		buff.Write(tag.EncodeContextTag())
		buff.Write(high)
	}

	if hasObjectId {
		objectIDBytes, err := p.ObjectId.MarshalBinary()
		if err != nil {
			return nil, err
		}
		tag.TagNumber = 2
		tag.LenValue = len(objectIDBytes)
		buff.Write(tag.EncodeContextTag())
		buff.Write(objectIDBytes)
	} else {
		cs := types.CharacterString{Value: p.ObjectName}
		nameBytes, err := cs.MarshalBinary()
		if err != nil {
			return nil, err
		}
		tag.TagNumber = 3
		tag.LenValue = len(nameBytes)
		buff.Write(tag.EncodeContextTag())
		buff.Write(nameBytes)
	}

	return buff.Bytes(), nil
}

func (p *WhoHas) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	offset := 0
	tag := &types.Tag{}
	headerLength := tag.DecodeTag(b)
	if headerLength == 0 {
		return errors.New("malformed who-has request")
	}

	hasRange := false
	var low, high uint32

	if tag.IsContext(0) && !tag.Opening && !tag.Closing {
		if tag.LenValue < 1 || tag.LenValue > 4 || len(b)-offset-headerLength < tag.LenValue {
			return errors.New("device instance range low limit is truncated")
		}
		low = types.DecodeVarUint(b[offset+headerLength : offset+headerLength+tag.LenValue])
		offset += headerLength + tag.LenValue

		if offset >= len(b) {
			return errors.New("who-has range is missing a high limit")
		}
		highTag := &types.Tag{}
		highHeader := highTag.DecodeTag(b[offset:])
		if highHeader == 0 || !highTag.IsContext(1) || highTag.Opening || highTag.Closing {
			return errors.New("expected a device instance range high limit")
		}
		if highTag.LenValue < 1 || highTag.LenValue > 4 || len(b)-offset-highHeader < highTag.LenValue {
			return errors.New("device instance range high limit is truncated")
		}
		high = types.DecodeVarUint(b[offset+highHeader : offset+highHeader+highTag.LenValue])
		offset += highHeader + highTag.LenValue
		hasRange = true

		if offset >= len(b) {
			return errors.New("who-has is missing an object selector")
		}
		tag = &types.Tag{}
		headerLength = tag.DecodeTag(b[offset:])
		if headerLength == 0 {
			return errors.New("malformed who-has request")
		}
	}

	var objectId *types.ObjectId
	var objectName string

	switch {
	case tag.IsContext(2) && !tag.Opening && !tag.Closing:
		if tag.LenValue != 4 || len(b)-offset-headerLength < tag.LenValue {
			return errors.New("object identifier is truncated")
		}
		objectId = &types.ObjectId{}
		if err := objectId.UnmarshalBinary(b[offset+headerLength : offset+headerLength+tag.LenValue]); err != nil {
			return err
		}
		offset += headerLength + tag.LenValue

	case tag.IsContext(3) && !tag.Opening && !tag.Closing:
		if tag.LenValue < 1 || len(b)-offset-headerLength < tag.LenValue {
			return errors.New("object name is truncated")
		}
		cs := types.CharacterString{}
		if err := cs.UnmarshalBinary(b[offset+headerLength : offset+headerLength+tag.LenValue]); err != nil {
			return err
		}
		objectName = cs.Value
		offset += headerLength + tag.LenValue

	default:
		return errors.New("expected an object identifier or object name")
	}

	if offset != len(b) {
		return errors.New("unexpected trailing data after who-has request")
	}

	p.HasRange = hasRange
	p.LowLimit = low
	p.HighLimit = high
	p.ObjectId = objectId
	p.ObjectName = objectName
	return nil
}

// IHave is the I-Have-Request payload (ASHRAE 135 §16.9): the identity of
// the responding device, the object it found, and that object's name. All
// three fields are application-tagged, in order.
type IHave struct {
	DeviceId   types.ObjectId
	ObjectId   types.ObjectId
	ObjectName string
}

func (p *IHave) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	deviceId := types.PropertyValue{Type: types.TagObjectId, Value: p.DeviceId}
	encoded, err := deviceId.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)

	objectId := types.PropertyValue{Type: types.TagObjectId, Value: p.ObjectId}
	encoded, err = objectId.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)

	name := types.PropertyValue{Type: types.TagCharacterString, Value: types.CharacterString{Value: p.ObjectName}}
	encoded, err = name.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)

	return buff.Bytes(), nil
}

func (p *IHave) UnmarshalBinary(b []byte) error {
	values, err := decodePropertyResultValues(b)
	if err != nil {
		return err
	}
	if len(values) != 3 {
		return errors.New("i-have expects exactly three application-tagged values")
	}

	deviceIdVal, ok := values[0].Value.(types.ObjectId)
	if !ok || values[0].Type != types.TagObjectId {
		return errors.New("expected a device object identifier")
	}
	objectIdVal, ok := values[1].Value.(types.ObjectId)
	if !ok || values[1].Type != types.TagObjectId {
		return errors.New("expected an object identifier")
	}
	nameVal, ok := values[2].Value.(types.CharacterString)
	if !ok || values[2].Type != types.TagCharacterString {
		return errors.New("expected an object name")
	}

	p.DeviceId = deviceIdVal
	p.ObjectId = objectIdVal
	p.ObjectName = nameVal.Value
	return nil
}
