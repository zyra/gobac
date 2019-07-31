package gobac

import (
	"errors"
	"github.com/zyra/gobac/types"
	"net"
)

type Device struct {
	*Object
	Server          *Server
	IPAddress       *net.IP
	MACAddress      *net.HardwareAddr
	DeviceID        uint32
	MaxAPDU         uint32
	OriginInterface string
	VendorID        uint16
	Segmentation    types.BACNET_SEGMENTATION
}

func NewDevice() *Device {
	d := Device{
		Object: &Object{},
	}
	d.IsDevice = true
	d.Device = &d
	return &d
}

func (d *Device) GetObjects(dest *[]*Object) error {
	prop, err := d.GetProperty(types.PROP_OBJECT_LIST)

	if err != nil {
		return err
	}

	if prop.Values == nil {
		return errors.New("property value is nil")
	}

	prop.ReadValueAsObject(dest)
	return nil
}
