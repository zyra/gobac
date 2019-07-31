package gobac

import (
	"errors"
	"fmt"
	"time"
)

type readPropertyRequest struct {
	*Request
	object     *Object
	propertyId PropertyId
}

func (s *Server) SendReadPropertyRequest(object *Object, propertyId PropertyId, dest *Property) error {
	req := &readPropertyRequest{
		Request:    NewRequest(s),
		propertyId: propertyId,
		object:     object,
	}

	req.InvokeID = NewTransaction()
	tc, c, h := getChanHandlerWithTimeout(time.Second * 30)
	s.setConfirmedHandler(req.InvokeID, h)

	defer ReleaseTransaction(req.InvokeID)
	defer s.removeConfirmedHandler(req.InvokeID)

	req.ExpectingReply = true
	req.IsBroadcastTarget = false
	req.EncodeNpdu()
	req.EncodeReadPropertyApdu()

	if req.Len() >= int(object.Device.MaxAPDU) {
		return errors.New("request size exceeds destination's max APDU")
	}

	req.Target = object.Device.IPAddress

	req.Send()

	select {
	case <-tc:
		return nil

	case data := <-c:
		if err := data.DecodeReadPropertyApdu(object, propertyId, dest); err != nil {
			return err
		}
	}

	return nil
}

func (d *readPropertyRequest) EncodeReadPropertyApdu() {
	_ = d.AppendByte(PduTypeConfirmedServiceRequest)
	_ = d.AppendByte(5)
	_ = d.AppendByte(d.InvokeID)
	_ = d.AppendByte(ConfirmedServiceReadProperty)

	_ = d.EncodeTag(0, true, 4)
	_ = d.EncodeObjectId(d.object.Type, d.object.Instance)

	var lenValue uint32 = 1

	if d.propertyId > 0x100 {
		lenValue++
	}

	_ = d.EncodeTag(1, true, lenValue)
	_ = d.AppendByte(uint8(d.propertyId))
}

func (r *Response) DecodeReadPropertyApdu(object *Object, propertyId PropertyId, dest *Property) error {
	// Check object id + instance
	tagNumber, lenValue := r.DecodeTag()

	if tagNumber != TagContextObjectId {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextObjectId, tagNumber)
	}

	objectType, objectInstance := r.DecodeObjectId()

	if objectType != object.Type {
		return fmt.Errorf("expected object type to be %d but got %d", object.Type, objectType)
	}

	if objectInstance != object.Instance {
		return fmt.Errorf("expected object instance to be %d but got %d", object.Instance, objectInstance)
	}

	// Check property id
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != TagContextPropertyId {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextPropertyId, tagNumber)
	}

	propId := uint16(r.DecodeUnsigned(lenValue))

	if propId != propertyId {
		return fmt.Errorf("expected propertyId to be %d but got %d", propertyId, propId)
	}

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
		value := &PropertyValue{}
		r.DecodePropertyValue(value)
		values = append(values, value)
	}

	dest.Values = &values

	// Closing tag
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != TagContextPropertyValue {
		return fmt.Errorf("expected tagNumber to be %d but got %d", TagContextPropertyValue, tagNumber)
	}

	return nil
}

func (r *Response) DecodePropertyValue(dest *PropertyValue) {
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
		dest.Value = r.DecodeCharacterString(lenValue)
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
		value := &ObjectIdValue{
			Type:     objectType,
			Instance: objectInstance,
		}
		dest.Value = value
	}
}
