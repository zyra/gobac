package types

import (
	"net"
)

type Object struct {
	IPAddress    *net.IP
	ObjectId     *ObjectId
	PresentValue *PropertyValue // TODO revise this
	Description  string
	DeviceID     Uint16
}

func (o *Object) Reset() {
	o.IPAddress = nil
	o.ObjectId = nil
	o.PresentValue = nil
	o.Description = ""
	o.DeviceID = 0
}
