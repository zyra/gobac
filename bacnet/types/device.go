package types

import (
	"bytes"
	"errors"
	"net"
)

type Device struct {
	Object
	MACAddress      net.HardwareAddr
	MaxAPDU         Uint32
	OriginInterface string
	VendorID        Uint16
	Segmentation    Segmentation
}

func (d *Device) UnmarshalBinary(data []byte) error {
	buff := bytes.NewBuffer(data)

	t := &Tag{}

	//
	// Decode device ID
	//

	// Get tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	// Check tag number
	if t.TagNumber != ApplicationTagObjectId {
		return errors.New("invalid object id application tag")
	}

	d.ObjectId = &ObjectId{}

	// Decode device ID
	if err := d.ObjectId.UnmarshalBinary(buff.Next(t.LenValue)); err != nil {
		return err
	}

	// make sure the type is correct
	if d.ObjectId.Type != ObjectTypeDevice {
		return errors.New("object type isn't a device")
	}

	//
	// Decode max APDU
	//

	// Get tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	// Check tag number
	if t.TagNumber != ApplicationTagUnsignedInt {
		return errors.New("invalid apdu application tag")
	}

	d.MaxAPDU = Uint32(DecodeVarUint(buff.Next(t.LenValue)))

	//
	// Decode segmentation
	//

	// Get tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	// Check tag number
	if t.TagNumber != ApplicationTagEnumerated {
		return errors.New("invalid segmentation application tag")
	}

	d.Segmentation = Segmentation(DecodeVarUint(buff.Next(t.LenValue)))

	if d.Segmentation >= SegmentationMax {
		return errors.New("invalid segmentation value")
	}

	//
	// Decode VendorID
	//

	// Get tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	// Check tag number
	if t.TagNumber != ApplicationTagUnsignedInt {
		return errors.New("invalid vendor id application tag")
	}

	d.VendorID = Uint16(DecodeVarUint(buff.Next(t.LenValue)))

	if d.VendorID > 0xFFFF {
		return errors.New("invalid vendor id")
	}

	return nil
}
