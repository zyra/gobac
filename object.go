package gobac

import "github.com/zyra/gobac/types"

type Object struct {
	Device       *Device
	Type         types.ObjectType
	Instance     uint16
	PresentValue *PropertyValue
	Description  string
}

func (o *Object) GetProperty(id PropertyId) (prop *Property, err error) {
	prop = &Property{
		Object: o,
	}

	prop.Index = 0
	prop.ID = id

	if err = prop.GetValue(); err != nil {
		return prop, err
	}

	return prop, nil
}

func (o *Object) GetAllProperties() (*[]*Property, error) {
	properties := make([]*Property, 0)

	return &properties, nil
}
