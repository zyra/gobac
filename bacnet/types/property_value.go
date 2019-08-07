package types

import (
	"bytes"
	"fmt"
	"reflect"
)

var typeOfFloat = reflect.TypeOf(float64(0))
var typeOfFloat32 = reflect.TypeOf(float32(0))
//var typeOfInt = reflect.TypeOf(int64(0))
var typeOfInt32 = reflect.TypeOf(int32(0))
//var typeOfUnt = reflect.TypeOf(uint64(0))
var typeOfUint32 = reflect.TypeOf(uint32(0))
var typeOfString = reflect.TypeOf("")

func invalidVal(p *PropertyValue) error {
	return fmt.Errorf("invalid value %x for type %x", p.Value, p.Type)
}

type PropertyValue struct {
	Type  DataType
	Value interface{}
}

func (p *PropertyValue) ReadAsObject() Object {
	obj := Object{}
	if v, ok := p.Value.(ObjectId); ok {
		obj.ObjectId = &v
	}
	return obj
}

func (p *PropertyValue) IsNumeric() bool {
	return reflect.TypeOf(p.Value).ConvertibleTo(typeOfFloat)
}

func (p *PropertyValue) ReadAsFloat64() float64 {
	if p.IsNumeric() {
		return reflect.ValueOf(p.Value).Convert(typeOfFloat).Float()
	}
	return 0
}

func (p *PropertyValue) ReadAsFloat64Unsafe() float64 {
	return reflect.ValueOf(p.Value).Convert(typeOfFloat).Float()
}

func (p *PropertyValue) ReadAsString() string {
	if p.Value == nil {
		return "NULL"
	}

	switch p.Value.(type) {
	case CharacterString:
		return p.Value.(CharacterString).Value
	case string:
		return p.Value.(string)

	case bool:
		if p.Value.(bool) == true {
			return "true"
		} else {
			return "false"
		}

	case uint32, int32:
		return fmt.Sprintf("%d", p.Value)

	case float32, float64, Real, Double:
		return fmt.Sprintf("%f", p.Value)

	case BitString:
		return fmt.Sprintf("%b", p.Value)

	case []byte:
		return fmt.Sprintf("%x", p.Value)

	case fmt.Stringer:
		return p.Value.(fmt.Stringer).String()
	}

	if reflect.TypeOf(p.Value).ConvertibleTo(typeOfString) {
		return reflect.ValueOf(p.Value).Convert(typeOfString).String()
	} else {
		return fmt.Sprintf("%s", p.Value)
	}
}

func (p *PropertyValue) ValueLength(b []byte) (length int, bytesRead int) {
	t := GetTag()
	defer t.Release()
	bytesRead = t.DecodeTag(b)
	return t.LenValue, bytesRead
}

func (p *PropertyValue) MarshalBinary() (b []byte, err error) {
	buff := bytes.NewBuffer([]byte{})

	tag := GetTag()
	defer tag.Release()

	tag.TagNumber = p.Type

	switch p.Type {
	case TagNull:
		break

	case TagBoolean:
		if bVal, ok := p.Value.(bool); ok && bVal {
			tag.LenValue = 1
		}

		if _, err := buff.Write(tag.EncodeTag()); err != nil {
			return nil, err
		}
		break

	case TagUnsigned,
		TagEnumerated:
		if _, ok := p.Value.(uint32); !ok {
			if !reflect.TypeOf(p.Value).ConvertibleTo(typeOfUint32) {
				return nil, invalidVal(p)
			}

			p.Value = uint32(reflect.ValueOf(p.Value).Convert(typeOfUint32).Uint())
		}


		uintBytes := EncodeVarUint(p.Value.(uint32))

		tag.LenValue = len(uintBytes)

		if _, err := buff.Write(tag.EncodeTag()); err != nil {
			return nil, err
		} else if _, err := buff.Write(uintBytes); err != nil {
			return nil, err
		}
		break

	case TagSigned:
		if _, ok := p.Value.(int32); !ok {
			if !reflect.TypeOf(p.Value).ConvertibleTo(typeOfInt32) {
				return nil, invalidVal(p)
			}

			p.Value = int32(reflect.ValueOf(p.Value).Convert(typeOfInt32).Int())
		}

		uintBytes := EncodeVarInt(p.Value.(int32))
		tag.LenValue = len(uintBytes)

		if _, err := buff.Write(tag.EncodeTag()); err != nil {
			return nil, err
		} else if _, err := buff.Write(uintBytes); err != nil {
			return nil, err
		}
		break

	case TagReal:
		if _, ok := p.Value.(float32); !ok {
			if !reflect.TypeOf(p.Value).ConvertibleTo(typeOfFloat32) {
				return nil, invalidVal(p)
			}

			p.Value = float32(reflect.ValueOf(p.Value).Convert(typeOfFloat32).Float())
		}

		tag.LenValue = 4

		if _, err := buff.Write(tag.EncodeTag()); err != nil {
			return nil, err
		}

		if realBytes, err := Real(p.Value.(float32)).MarshalBinary(); err != nil {
			return nil, err
		} else if _, err := buff.Write(realBytes); err != nil {
			return nil, err
		}

		break

	case TagDouble:
		if _, ok := p.Value.(float64); !ok {
			if !reflect.TypeOf(p.Value).ConvertibleTo(typeOfFloat) {
				return nil, invalidVal(p)
			}

			p.Value = reflect.ValueOf(p.Value).Convert(typeOfFloat).Float()
		}

		tag.LenValue = 8

		if _, err := buff.Write(tag.EncodeTag()); err != nil {
			return nil, err
		}

		if doubleBytes, err := Double(p.Value.(float64)).MarshalBinary(); err != nil {
			return nil, err
		} else if _, err := buff.Write(doubleBytes); err != nil {
			return nil, err
		}

		break

	case TagOctetString:
		if byteVal, ok := p.Value.([]byte); ok {
			tag.LenValue = 4
			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(byteVal); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	case TagCharacterString:
		if csVal, ok := p.Value.(CharacterString); ok {
			tag.LenValue = csVal.Length()
			csBytes, err := csVal.MarshalBinary()

			if err != nil {
				return nil, err
			}

			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(csBytes); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	case TagBitString:
		if byteVal, ok := p.Value.(BitString); ok {
			tag.LenValue = len(byteVal)

			bsBytes, err := byteVal.MarshalBinary()

			if err != nil {
				return nil, err
			}

			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(bsBytes); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	case TagDate:
		if dateVal, ok := p.Value.(*Date); ok {
			tag.LenValue = 4
			dateBytes, err := dateVal.MarshalBinary()

			if err != nil {
				return nil, err
			}

			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(dateBytes); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	case TagTime:
		if timeVal, ok := p.Value.(*Time); ok {
			tag.LenValue = 4

			timeBytes, err := timeVal.MarshalBinary()

			if err != nil {
				return nil, err
			}

			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(timeBytes); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	case TagObjectId:
		if objIdVal, ok := p.Value.(*ObjectId); ok {
			tag.LenValue = 4
			objIdBytes, err := objIdVal.MarshalBinary()

			if err != nil {
				return nil, err
			}

			if _, err := buff.Write(tag.EncodeTag()); err != nil {
				return nil, err
			} else if _, err := buff.Write(objIdBytes); err != nil {
				return nil, err
			}
		} else {
			return nil, invalidVal(p)
		}
		break

	}

	return buff.Bytes(), nil
}

func (p *PropertyValue) UnmarshalBinary(b []byte) (err error) {
	var t *Tag
	var r int

	t = GetTag()
	defer t.Release()
	r = t.DecodeTag(b)
	p.Type = t.TagNumber

	switch p.Type {
	case TagNull:
		return nil

	case TagBoolean:
		p.Value = t.LenValue != 0
		return nil

	case TagUnsigned, TagEnumerated:
		p.Value = DecodeVarUint(b[r:])
		return nil

	case TagSigned:
		p.Value = DecodeVarInt(b[r:])
		return nil

	case TagReal:
		val := Real(0)
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagDouble:
		val := Double(0)
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagOctetString:
		p.Value = b[r:]
		return nil

	case TagCharacterString:
		val := CharacterString{}
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagBitString:
		val := BitString{}
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagDate:
		val := Date{}
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagTime:
		val := Time{}
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err

	case TagObjectId:
		val := ObjectId{}
		err = val.UnmarshalBinary(b[r:])
		p.Value = val
		return err
	}

	return nil
}
