package gobac

import (
	"errors"
	"fmt"
	"github.com/zyra/gobac/types"
	"net"
	"time"
)

type WritePropertyRequest struct {
	ConfirmedRequest
	propertyId     PropertyId
	tag            DataTag
	value          interface{}
	objectType     uint16
	objectInstance uint16
}

func (s *Server) SendWritePropertyRequest(deviceAddress net.IP, objectType, objectInstance uint16, propertyId PropertyId, tag DataTag, priority uint8, value interface{}) error {
	req := &WritePropertyRequest{
		ConfirmedRequest: s.NewConfirmedRequest(),
		propertyId:       propertyId,
		tag:              tag,
		value:            value,
		objectType:       objectType,
		objectInstance:   objectInstance,
	}

	defer req.Cleanup()

	req.Priority = priority
	req.Target = deviceAddress

	if err := req.EncodeWritePropertyApdu(); err != nil {
		return err
	}

	if err := req.Send(); err != nil {
		return err
	}

	tc := time.After(s.DefaultTimeout)

	select {
	case <-tc:
		return errors.New("DefaultTimeout")

	case data := <-req.Data():
		if data.Failed {
			if data.Errored {
				return fmt.Errorf("error with %s: %s", data.ErrorClassString, data.ErrorCodeString)
			}

			if data.Aborted {
				return fmt.Errorf("aborted: %s", data.AbortReasonString)
			}

			if data.Rejected {
				return fmt.Errorf("rejected: %s", data.RejectReasonString)
			}

			return errors.New("unknown failure reason")
		}

		return nil
	}
}

func (d *WritePropertyRequest) EncodeWritePropertyApdu() (err error) {
	err = d.AppendByte(PduTypeConfirmedServiceRequest)
	err = d.AppendByte(5)
	err = d.AppendByte(d.InvokeID)
	err = d.AppendByte(ConfirmedServiceWriteProperty)

	err = d.EncodeTag(TagContextObjectId, true, 4)
	err = d.EncodeObjectId(d.objectType, d.objectInstance)

	lenValue := getUnsignedLen(uint(d.propertyId))

	err = d.EncodeTag(TagContextPropertyId, true, lenValue)
	err = d.EncodeUnsigned(uint32(d.propertyId))

	//
	// Set index
	//
	// Index can only be set on array types
	// Let's check the property type first to see if it's an array type
	switch d.propertyId {
	case PropPriorityArray,
		PropEventTimeStamps,
		PropAction,
		PropObjectList,
		PropListOfObjectPropertyReferences,
		PropNegativeAccessRules,
		PropPositiveAccessRules,
		PropShedLevelDescriptions,
		PropShedLevels,
		PropAccessDoors,
		PropAuthenticationFactors,
		PropAssignedAccessRights,
		PropSupportedFormatClasses,
		PropSupportedFormats,
		PropStateChangeValues,
		PropSubordinateNodeTypes,
		PropProtocolObjectTypesSupported,
		PropWeeklySchedule:
		// these guys can have an index
		// set index to 1
		// TODO make index configurable
		err = d.EncodeTag(TagContextPropertyArrayIndex, true, 1)
		err = d.AppendByte(0x1)
		break
	}

	err = d.EncodeOpeningTag(TagContextPropertyValue)

	err = d.EncodeData()

	err = d.EncodeClosingTag(TagContextPropertyValue)

	if d.Priority > 0 {
		err = d.EncodeTag(4, true, 1)
		err = d.AppendByte(d.Priority)
	}

	return err
}

func (d *WritePropertyRequest) EncodeData() (err error) {
	invalidVal := fmt.Errorf("invalid value %x for type %x", d.value, d.tag)

	switch d.tag {
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
			err = d.EncodeReal(float32(d.value.(float64)))
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
