package gobac

import (
	"errors"
	"fmt"
	"github.com/zyra/gobac/types"
)

type writePropertyRequest struct {
	*Request
	object       *Object
	propertyId   PropertyId
	propertyType DataTag
	value        interface{}
}

func (s *Server) SendWritePropertyRequest(object *Object,
	propertyId PropertyId,
	propertyType DataTag,
	priority uint8,
	value interface{}) error {
	req := &writePropertyRequest{
		Request:      NewRequest(s),
		propertyId:   propertyId,
		object:       object,
		propertyType: propertyType,
		value:        value,
	}

	req.Priority = priority

	req.InvokeID = NewTransaction()
	defer ReleaseTransaction(req.InvokeID)

	req.ExpectingReply = true
	req.IsBroadcastTarget = false
	req.EncodeNpdu()
	if err := req.EncodeWritePropertyApdu(); err != nil {
		return err
	}

	if req.Len() >= int(object.Device.MaxAPDU) {
		return errors.New("request size exceeds destination's max APDU")
	}

	req.Target = object.Device.IPAddress

	//tc, c, h := getChanHandlerWithTimeout(time.Second * 30)
	//s.setConfirmedHandler(req.InvokeID, h)
	//defer s.removeConfirmedHandler(req.InvokeID)

	req.Send()

	//select {
	//case <-tc:
	//	return nil
	//
	//case data := <-c:
	//	if data.Failed {
	//		switch data.PduType {
	//		case PduTypeError:
	//			return errors.New("device responded with error")
	//		case PduTypeAbort:
	//			return errors.New("device aborted request")
	//		case PduTypeReject:
	//			return errors.New("device rejected request")
	//		}
	//	}
	//
	//	return nil
	//}

	return nil
}

func (d *writePropertyRequest) EncodeWritePropertyApdu() (err error) {
	err = d.AppendByte(PduTypeConfirmedServiceRequest)
	err = d.AppendByte(5)
	err = d.AppendByte(d.InvokeID)
	err = d.AppendByte(ConfirmedServiceWriteProperty)

	err = d.EncodeTag(TagContextObjectId, true, 4)
	err = d.EncodeObjectId(d.object.Type, d.object.Instance)

	var lenValue uint32 = 1

	if d.propertyId > 0x100 {
		lenValue++
	}

	err = d.EncodeTag(TagContextPropertyId, true, lenValue)
	err = d.AppendByte(uint8(d.propertyId))

	err = d.EncodeOpeningTag(TagContextPropertyValue)

	err = d.EncodeData()

	err = d.EncodeClosingTag(TagContextPropertyValue)

	if d.Priority > 0 {
		err = d.EncodeTag(4, true, 1)
		err = d.AppendByte(d.Priority)
	}

	return err
}

func (d *writePropertyRequest) EncodeData() (err error) {
	invalidVal := fmt.Errorf("invalid value %x for type %x", d.value, d.propertyType)

	switch d.propertyType {
	case TagNull:
		break

	case TagBoolean:
		val := 0
		if d.value == true {
			val = 1
		}
		if err := d.EncodeTag(TagBoolean, false, uint32(val)); err != nil {
			return err
		}
		break

	case TagUnsigned,
		TagEnumerated:
		if uintVal, ok := d.value.(uint32); ok {
			l := getUnsignedLen(uint(uintVal))
			err = d.EncodeTag(TagUnsigned, false, l)
			if err := d.EncodeUnsigned(uintVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagSigned:
		if intVal, ok := d.value.(int32); ok {
			l := getSignedLen(int(intVal))
			err = d.EncodeTag(TagSigned, false, l)
			if err := d.EncodeSigned(intVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagReal:
		err = d.EncodeTag(TagReal, false, 4)
		switch d.value.(type) {
		case float32:
			err = d.EncodeReal(d.value.(float32))
			break

		case float64:
			err = d.EncodeReal(d.value.(float32))
			break

		case int8:
			err = d.EncodeReal(float32(d.value.(int8)))
			break

		case int16:
			err = d.EncodeReal(float32(d.value.(int16)))
			break

		case int32:
			err = d.EncodeReal(float32(d.value.(int32)))
			break

		case int64:
			err = d.EncodeReal(float32(d.value.(int64)))
			break

		case int:
			err = d.EncodeReal(float32(d.value.(int)))
			break

		case uint8:
			err = d.EncodeReal(float32(d.value.(uint8)))
			break

		case uint16:
			err = d.EncodeReal(float32(d.value.(uint16)))
			break

		case uint32:
			err = d.EncodeReal(float32(d.value.(uint32)))
			break

		case uint64:
			err = d.EncodeReal(float32(d.value.(uint64)))
			break

		case uint:
			err = d.EncodeReal(float32(d.value.(uint)))
			break
		default:
			return invalidVal
		}
		break

	case TagDouble:
		err = d.EncodeTag(TagReal, false, 8)
		switch d.value.(type) {
		case float32:
			err = d.EncodeDouble(float64(d.value.(float32)))
			break

		case float64:
			err = d.EncodeDouble(d.value.(float64))
			break

		case int8:
			err = d.EncodeDouble(float64(d.value.(int8)))
			break

		case int16:
			err = d.EncodeDouble(float64(d.value.(int16)))
			break

		case int32:
			err = d.EncodeDouble(float64(d.value.(int32)))
			break

		case int64:
			err = d.EncodeDouble(float64(d.value.(int64)))
			break

		case int:
			err = d.EncodeDouble(float64(d.value.(int)))
			break

		case uint8:
			err = d.EncodeDouble(float64(d.value.(uint8)))
			break

		case uint16:
			err = d.EncodeDouble(float64(d.value.(uint16)))
			break

		case uint32:
			err = d.EncodeDouble(float64(d.value.(uint32)))
			break

		case uint64:
			err = d.EncodeDouble(float64(d.value.(uint64)))
			break

		case uint:
			err = d.EncodeDouble(float64(d.value.(uint)))
			break
		default:
			return invalidVal
		}
		break

	case TagOctetString:
		if byteVal, ok := d.value.([]byte); ok {
			err = d.EncodeTag(TagOctetString, false, uint32(len(byteVal)))
			if err := d.AppendBytes(byteVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagCharacterString:
		if stringVal, ok := d.value.(string); ok {
			err = d.EncodeTag(TagOctetString, false, 1+uint32(len([]byte(stringVal))))
			if err := d.EncodeCharacterString(stringVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagBitString:
		if byteVal, ok := d.value.([]byte); ok {
			err = d.EncodeTag(TagOctetString, false, uint32(len(byteVal)))
			if err := d.EncodeBitString(byteVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagDate:
		if dateVal, ok := d.value.(*types.Date); ok {
			err = d.EncodeTag(TagOctetString, false, 4)
			if err := d.EncodeDate(dateVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagTime:
		if timeVal, ok := d.value.(*types.Time); ok {
			err = d.EncodeTag(TagOctetString, false, 4)
			if err := d.EncodeTime(timeVal); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	case TagObjectId:
		if objIdVal, ok := d.value.(*ObjectIdValue); ok {
			err = d.EncodeTag(TagOctetString, false, 4)
			if err := d.EncodeObjectId(objIdVal.Type, objIdVal.Instance); err != nil {
				return err
			}
		} else {
			return invalidVal
		}
		break

	}

	return err
}
