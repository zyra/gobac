package gobac

import "github.com/zyra/gobac/types"

type ObjectIdValue struct {
	Type     types.ObjectType
	Instance uint16
}

type Property struct {
	Object *Object
	ID     PropertyId
	Index  uint32
	Values []*PropertyValue
}

func (p *Property) ReadValue(dest interface{}) {
	dest = p.Values
}

func (p *Property) ReadValuesAsObjects() []*Object {
	dest := make([]*Object,0, len(p.Values))
	for _, val := range p.Values {
		v := val.ReadAsObject()
		v.Device = p.Object.Device
		dest = append(dest, &v)
	}
	return dest
}

func (p *Property) GetValue() error {
	deviceAddress := p.Object.Device.IPAddress
	objectType := p.Object.Type
	objectInstance := p.Object.Instance
	return p.Object.Device.Server.SendReadPropertyRequest(deviceAddress, objectType, objectInstance, p.ID, p)
}

func (p *Property) SetValue(dataType DataTag, value interface{}) error {
	deviceAddress := p.Object.Device.IPAddress
	objectType := p.Object.Type
	objectInstance := p.Object.Instance
	return p.Object.Device.Server.SendWritePropertyRequest(deviceAddress, objectType, objectInstance, p.ID, dataType, 8, value)
}
