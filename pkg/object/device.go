package object

import (
	_type "github.com/zyra/bacnet-2/pkg/type"
	"net"
)

type Device struct {
	IPAddress       *net.IP
	MACAddress      *net.HardwareAddr
	DeviceID        uint32
	MaxAPDU         uint32
	OriginInterface string
	VendorID        uint16
	Segmentation    _type.BACNET_SEGMENTATION
}
