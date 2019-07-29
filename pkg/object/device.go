package object

import (
	_type "github.com/zyra/bacnet-2/pkg/type"
	"net"
)

type Device struct {
	*Object
	IPAddress       *net.IP
	MACAddress      *net.HardwareAddr
	DeviceID        uint32
	MaxAPDU         uint32
	OriginInterface string
	VendorID        uint16
	Segmentation    _type.BACNET_SEGMENTATION
}

func NewDevice() *Device {
	var d Device
	d.IsDevice = true
	d.Device = &d
	return &d
}

func (d *Device) GetObjects() (*[]*Object, error) {
	objects := make([]*Object, 0)
	prop, err := d.GetProperty(_type.PROP_OBJECT_LIST, 0)

	if err != nil {
		return nil, err
	}

	prop.ReadValue(&objects)

	return &objects, err
}
