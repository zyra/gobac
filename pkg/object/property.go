package object

import (
	"github.com/zyra/bacnet-2/pkg/service"
	_type "github.com/zyra/bacnet-2/pkg/type"
)

type Property struct {
	ID        _type.PropertyId
	Index     uint32
	Value     interface{}
	ValueType _type.ApplicationTag
	Object    *Object
}

func (p *Property) ReadValue(dest interface{}) {
	dest = p.Value
}

func (p *Property) GetValue() error {
	return service.SendReadPropertyRequest(p.Object.Device, p.ID, p)
}

func (p *Property) SetValue(interface{}) error {
	return nil
}
