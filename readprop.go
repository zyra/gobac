package gobac

import (
	"errors"
	"fmt"
	"net"
	"time"
)

type ReadPropertyRequest struct {
	ConfirmedRequest
	propertyId     PropertyId
	objectType     uint16
	objectInstance uint16
}

func (s *Server) SendReadPropertyRequest(deviceAddress net.IP, objectType, objectInstance uint16, propertyId PropertyId, dest *Property) error {
	req := &ReadPropertyRequest{
		ConfirmedRequest: s.NewConfirmedRequest(),
		propertyId:       propertyId,
		objectType:       objectType,
		objectInstance:   objectInstance,
	}
	req.Target = deviceAddress

	defer req.Cleanup()

	if err := req.EncodeReadPropertyApdu(); err != nil {
		return err
	}

	if err := req.Send(); err != nil {
		return err
	}

	tc := time.After(s.DefaultTimeout)

	select {
	case <-tc:
		return errors.New("timeout")

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

		if prop, ok := data.Dest.(ReadPropertyResponse); ok {
			if prop.ObjectType != objectType {
				return fmt.Errorf("expected object type to be %d but got %d\n", objectType, prop.ObjectType)
			}

			if prop.ObjectInstance != objectInstance {
				return fmt.Errorf("expected object instance to be %d but got %d\n", objectInstance, prop.ObjectInstance)
			}

			if prop.ID != propertyId {
				return fmt.Errorf("expected property id to be %d but got %d\n", propertyId, prop.ID)
			}

			dest.ID = prop.ID
			dest.Values = prop.Values
		}
	}

	return nil
}

func (d *ReadPropertyRequest) EncodeReadPropertyApdu() (err error) {
	err = d.AppendByte(PduTypeConfirmedServiceRequest)
	err = d.AppendByte(5)
	err = d.AppendByte(d.InvokeID)
	err = d.AppendByte(ConfirmedServiceReadProperty)

	err = d.EncodeTag(0, true, 4)
	err = d.EncodeObjectId(d.objectType, d.objectInstance)

	var lenValue uint32 = 1

	if d.propertyId > 0x100 {
		lenValue++
	}

	err = d.EncodeTag(1, true, lenValue)
	err = d.AppendByte(uint8(d.propertyId))

	return err
}

type ReadPropertyResponse struct {
	ObjectType     uint16
	ObjectInstance uint16
	ID             uint16
	Values         []*PropertyValue
}

func (r *Response) DecodeReadPropertyApdu() error {
	dest := ReadPropertyResponse{}

	// Check object id + instance
	tagNumber, lenValue := r.DecodeTag()

	if tagNumber != TagContextObjectId {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextObjectId, tagNumber)
	}

	dest.ObjectType, dest.ObjectInstance = r.DecodeObjectId()

	// Check property id
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != TagContextPropertyId {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextPropertyId, tagNumber)
	}

	dest.ID = uint16(r.DecodeUnsigned(lenValue))

	// need to check array index here...
	// but since we omitted that from our request
	// and it's not available as an option anyway,
	// we're going to ignore the check and move on

	// Opening tag
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != TagContextPropertyValue {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextPropertyValue, tagNumber)
	}

	values := make([]*PropertyValue, 0)

	// Get properties
	for r.Len() > 1 {
		value := r.DecodePropertyValue()
		values = append(values, &value)
	}

	dest.Values = values

	// Closing tag
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != TagContextPropertyValue {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextPropertyValue, tagNumber)
	}

	r.Dest = dest

	return nil
}

func (r *Response) DecodePropertyValue() (dest PropertyValue) {
	tagNumber, lenValue := r.DecodeTag()
	dest.Type = tagNumber

	switch tagNumber {
	case TagNull: // nothing to do here
		break

	case TagBoolean:
		dest.Value = lenValue != 0
		break

	case TagUnsigned,
		TagEnumerated:
		dest.Value = r.DecodeUnsigned(lenValue)
		break

	case TagSigned:
		dest.Value = r.DecodeSigned(lenValue)
		break

	case TagReal:
		dest.Value = r.DecodeReal(lenValue)
		break

	case TagDouble:
		dest.Value = r.DecodeDouble(lenValue)
		break

	case TagOctetString:
		dest.Value = r.Next(int(lenValue))
		break

	case TagCharacterString:
		dest.Value = r.DecodeCharacterString(lenValue - 1)
		break

	case TagBitString:
		dest.Value = r.DecodeBitString(lenValue)
		break

	case TagDate:
		dest.Value = r.DecodeDate()
		break

	case TagTime:
		dest.Value = r.DecodeTime()
		break

	case TagObjectId:
		objectType, objectInstance := r.DecodeObjectId()
		value := ObjectIdValue{
			Type:     objectType,
			Instance: objectInstance,
		}
		dest.Value = value
	}

	return dest
}
