package gobac

import "github.com/zyra/gobac/types"

type Object struct {
	Device   *Device
	Type     types.ObjectType
	Instance uint16
	IsDevice bool
}

func (o *Object) GetProperty(id PropertyId) (*Property, error) {
	prop := &Property{
		Object: o,
	}

	prop.Index = 0
	prop.ID = id

	if err := prop.GetValue(); err != nil {
		return nil, err
	}

	return prop, nil
}

func (o *Object) GetAllProperties() (*[]*Property, error) {
	properties := make([]*Property, 0)

	return &properties, nil
}
