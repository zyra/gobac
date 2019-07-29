package gobac

import (
	"github.com/zyra/gobac/types"
)

type Property struct {
	ID        types.PropertyId
	Index     uint32
	Value     interface{}
	ValueType types.ApplicationTag
	Object    *Object
}

func (p *Property) ReadValue(dest interface{}) {
	dest = p.Value
}

func (p *Property) GetValue() error {
	return SendReadPropertyRequest(p.Object.Device, p.ID, p)
}

func (p *Property) SetValue(interface{}) error {
	return nil
}
