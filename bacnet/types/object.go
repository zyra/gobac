package types

import (
	"net"
)

type Object struct {
	IPAddress    net.IP
	ObjectId     *ObjectId
	PresentValue *PropertyValue // TODO revise this
	Name         string
	Description  string
	DeviceID     Uint16
	StateValues  map[string]string
}

func (o *Object) Reset() {
	o.IPAddress = net.IPv4zero
	o.ObjectId = nil
	o.PresentValue = nil
	o.Description = ""
	o.Name = ""
	o.DeviceID = 0
	o.StateValues = map[string]string{}
}
