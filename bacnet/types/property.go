package types

import (
	"bytes"
	"errors"
	"math"
	"net"
)

type Property struct {
	IPAddress *net.IP
	ObjectId  *ObjectId
	ID        PropertyId
	Index     uint32
	Values    []*PropertyValue
	Length    int
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

func (p *Property) MarshalBinary() (b []byte, err error) {
	buff := bytes.NewBuffer([]byte{})

	t := GetTag()
	defer t.Release()

	if p.ObjectId != nil {
		// encode object ID
		t.TagNumber = 0
		t.LenValue = 4
		if _, err = buff.Write(t.EncodeContextTag()); err != nil {
			return nil, err
		}

		objIdBytes, err := p.ObjectId.MarshalBinary()

		if _, err = buff.Write(objIdBytes); err != nil {
			return nil, err
		}
	}

	t.TagNumber = 1
	t.LenValue = GetUintLen(uint(p.ID))

	// write property ID tag
	if _, e := buff.Write(t.EncodeContextTag()); e != nil {
		return nil, e
	}

	// write property ID
	if _, e := buff.Write(EncodeVarUint(p.ID)); e != nil {
		return nil, e
	}

	if p.Values != nil && len(p.Values) > 0 {
		// set index if property is an array
		if isPropArray(p.ID) {
			t.TagNumber = 2
			t.LenValue = GetUintLen(uint(p.Index))
			if _, err = buff.Write(t.EncodeContextTag()); err != nil {
				return nil, err
			}
			if _, err = buff.Write(EncodeVarUint(p.Index)); err != nil {
				return nil, err
			}
		}

		// opening tag
		t.TagNumber = 3
		t.LenValue = 0

		if _, err = buff.Write(t.EncodeOpeningTag()); err != nil {
			return nil, err
		}

		// write property value
		if b, err := p.Values[0].MarshalBinary(); err != nil {
			return nil, err
		} else {
			if _, err = buff.Write(b); err != nil {
				return nil, err
			}
		}

		// closing tag
		if _, err = buff.Write(t.EncodeClosingTag()); err != nil {
			return nil, err
		}
	}

	return buff.Bytes(), nil
}

func (p *Property) UnmarshalBinary(b []byte) (err error) {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	var tagStart uint8
	var full bool
	var buff *bytes.Buffer

	buff = GetBuff(b...)
	defer ReleaseBuff(buff)

	full = p.ObjectId == nil

	t := GetTag()
	defer t.Release()
	tagStart = uint8(0)

	if full {
		// ObjectID is typically left marshaled
		// except for cov notifications
		// since the format there is different
		// and the response might have multiple properties
		// probably will be the case for read-property-multiple (not implemented)

		//
		// Decode Object ID
		//
		buff.Next(t.DecodeTag(buff.Bytes()))

		if !t.IsContext(0) {
			return errors.New("unexpected tag number")
		}

		tagStart = 1

		p.ObjectId = &ObjectId{}

		if err = p.ObjectId.UnmarshalBinary(buff.Next(t.LenValue)); err != nil {
			return err
		}
	}

	//
	// Decode Property ID
	//
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(tagStart) {
		return errors.New("unexpected tag number")
	}

	p.ID = DecodeVarUint(buff.Next(t.LenValue))

	//
	// Decode array index
	//
	r := t.DecodeTag(buff.Bytes())

	if !t.IsContext(tagStart + 1) {
		p.Index = math.MaxUint32
	} else {
		// mark bytes as read
		buff.Next(r)

		// decode index
		p.Index = DecodeVarUint(buff.Next(t.LenValue))
	}

	// Check opening tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(tagStart + 2) {
		return errors.New("unexpected tag number")
	}

	var l int
	var values []*PropertyValue

	for buff.Len() > 1 {
		//b := buff.Bytes()[0]
		//
		//if b&0x08 == 0x08 {
		//	// Context specific tag ahead
		//	// break the loop!
		//	break
		//}

		r = t.DecodeTag(buff.Bytes())
		if t.Context {
			break
		}

		val := PropertyValue{}

		l, r = val.ValueLength(buff.Bytes())

		if err = val.UnmarshalBinary(buff.Next(r + l)); err != nil {
			return err
		}
		values = append(values, &val)
	}

	// Check closing tag
	r = t.DecodeTag(buff.Bytes())

	if !t.IsContext(tagStart + 2) {
		return errors.New("unexpected tag number")
	} else if full {
		buff.Next(r)
	}

	p.Values = values
	p.Length = len(b) - buff.Len()

	return
}

func isPropArray(id PropertyId) bool {
	switch id {
	case PropertyPriorityArray,
		PropertyEventTimeStamps,
		PropertyAction,
		PropertyObjectList,
		PropertyListOfObjectPropertyReferences,
		PropertyNegativeAccessRules,
		PropertyPositiveAccessRules,
		PropertyShedLevelDescriptions,
		PropertyShedLevels,
		PropertyAccessDoors,
		PropertyAuthenticationFactors,
		PropertyAssignedAccessRights,
		PropertySupportedFormatClasses,
		PropertySupportedFormats,
		PropertyStateChangeValues,
		PropertySubordinateNodeTypes,
		PropertyProtocolObjectTypesSupported,
		PropertyWeeklySchedule:
		return true
	default:
		return false
	}
}
