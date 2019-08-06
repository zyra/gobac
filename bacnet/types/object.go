package types

import (
	"net"
)

type Object struct {
	IPAddress    net.IP
	ObjectId     *ObjectId
	PresentValue *PropertyValue // TODO revise this
	Description  string
	DeviceID     uint16
}
