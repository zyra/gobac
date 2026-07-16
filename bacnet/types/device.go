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
	Port            uint16
}

func NewDevice() *Device {
	return devPool.Get().(*Device)
}

func (d *Device) Reset() {
	d.Object.Reset()
	d.MACAddress = nil
	d.MaxAPDU = 0
	d.OriginInterface = ""
	d.VendorID = 0
	d.Segmentation = 0
	d.Port = 0
}

func (d *Device) Release() {
	d.Reset()
	devPool.Put(d)
}

func (d *Device) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("received an empty byte slice")
	}

	offset := 0
	readApplicationValue := func(number uint8, minLength, maxLength int) ([]byte, error) {
		if offset >= len(data) {
			return nil, errors.New("I-Am data is truncated")
		}
		tag := &Tag{}
		headerLength := tag.DecodeTag(data[offset:])
		if headerLength == 0 || tag.Context || tag.Opening || tag.Closing || tag.TagNumber != number {
			return nil, errors.New("invalid I-Am application tag")
		}
		if tag.LenValue < minLength || tag.LenValue > maxLength || len(data)-offset-headerLength < tag.LenValue {
			return nil, errors.New("invalid I-Am application value length")
		}
		offset += headerLength
		value := data[offset : offset+tag.LenValue]
		offset += tag.LenValue
		return value, nil
	}

	objectID, err := readApplicationValue(ApplicationTagObjectId, 4, 4)
	if err != nil {
		return err
	}
	d.ObjectId = &ObjectId{}
	if err := d.ObjectId.UnmarshalBinary(objectID); err != nil {
		return err
	}
	d.DeviceID = d.ObjectId.Instance
	d.DeviceInstance = d.ObjectId.InstanceNumber()
	if d.ObjectId.Type != ObjectTypeDevice {
		return errors.New("object type isn't a device")
	}

	maxAPDU, err := readApplicationValue(ApplicationTagUnsignedInt, 1, 4)
	if err != nil {
		return err
	}
	d.MaxAPDU = Uint32(DecodeVarUint(maxAPDU))

	segmentation, err := readApplicationValue(ApplicationTagEnumerated, 1, 4)
	if err != nil {
		return err
	}
	d.Segmentation = Segmentation(DecodeVarUint(segmentation))
	if d.Segmentation >= SegmentationMax {
		return errors.New("invalid segmentation value")
	}

	vendorID, err := readApplicationValue(ApplicationTagUnsignedInt, 1, 2)
	if err != nil {
		return err
	}
	d.VendorID = Uint16(DecodeVarUint(vendorID))
	if offset != len(data) {
		return errors.New("unexpected trailing I-Am data")
	}
	return nil
}
