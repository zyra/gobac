package gobac

import (
	"errors"
	"github.com/zyra/gobac/types"
	"net"
)

type Device struct {
	Server          *Server
	IPAddress       net.IP
	MACAddress      net.HardwareAddr
	DeviceID        uint16
	MaxAPDU         uint32
	OriginInterface string
	VendorID        uint16
	Segmentation    types.BACNET_SEGMENTATION
}

func (d *Device) GetObjects() (dest []*Object, err error) {
	obj := &Object{
		Device:   d,
		Instance: d.DeviceID,
		Type:     8,
	}

	prop, err := obj.GetProperty(types.PROP_OBJECT_LIST)

	if err != nil {
		return nil, err
	}

	if prop.Values == nil {
		return nil, errors.New("property value is nil")
	}

	dest = prop.ReadValuesAsObjects()

	return dest, nil
}
