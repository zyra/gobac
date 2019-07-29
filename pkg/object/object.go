package object

import (
	_type "github.com/zyra/bacnet-2/pkg/type"
)

type Object struct {
	Device   *Device
	Type     uint32
	Instance uint32
	IsDevice bool
}

func (o *Object) GetProperty(id _type.PropertyId, index uint32) (*Property, error) {
	prop := &Property{
		Object: o,
		Index:  index,
		ID:     id,
	}

	if err := prop.GetValue(); err != nil {
		return nil, err
	}

	return prop, nil
}

func (o *Object) GetAllProperties() (*[]*Property, error) {
	properties := make([]*Property, 0)

	return &properties, nil
}
