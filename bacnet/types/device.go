package types

import (
	"errors"
	"net"
	"sync"
)

var devPool = sync.Pool{
	New: func() interface{} {
		return new(Device)
	},
}

type Device struct {
	Object
	MACAddress      *net.HardwareAddr
	MaxAPDU         Uint32
	OriginInterface string
	VendorID        Uint16
	Segmentation    Segmentation
}

func NewDevice() *Device {
	return devPool.Get().(*Device)
}

func (d *Device) Reset() {
	d.Object.Reset()
	d.MACAddress = nil
	d.MaxAPDU = 0
	d.VendorID = 0
	d.Segmentation = 0
}

func (d *Device) Release() {
	d.Reset()
	devPool.Put(d)
}

func (d *Device) UnmarshalBinary(data []byte) error {
	if data == nil {
		return errors.New("received a nil byte slice")
	}

	if len(data) == 0 {
		return errors.New("received an empty byte slice")
	}

	buff := GetBuff(data...)
	defer ReleaseBuff(buff)

	t := GetTag()
	defer t.Release()

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

	d.DeviceID = d.ObjectId.Instance

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
