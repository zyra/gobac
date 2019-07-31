package gobac

import "github.com/zyra/gobac/types"

type PropertyValue struct {
	Type  DataTag
	Value interface{}
}

type ObjectIdValue struct {
	Type     types.ObjectType
	Instance uint16
}

type Property struct {
	ID     PropertyId
	Index  uint32
	Values *[]*PropertyValue
	Object *Object
}

func (p *Property) ReadValue(dest interface{}) {
	dest = p.Values
}

func (p *Property) ReadValueAsObject(dest *[]*Object) {
	for _, val := range *p.Values {
		v := val.Value.(*ObjectIdValue)

		obj := &Object{
			Type:     v.Type,
			Instance: v.Instance,
			IsDevice: v.Type == types.OBJECT_DEVICE,
			Device:   p.Object.Device,
		}

		*dest = append(*dest, obj)
	}
}

func (p *Property) GetValue() error {
	return p.Object.Device.Server.SendReadPropertyRequest(p.Object, p.ID, p)
}

func (p *Property) SetValue(dataType DataTag, value interface{}) error {
	return p.Object.Device.Server.SendWritePropertyRequest(p.Object, p.ID, dataType, 6, value)
}
