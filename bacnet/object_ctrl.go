package bacnet

import "github.com/zyra/gobac/bacnet/types"

type ObjectController types.Object

func (o *ObjectController) GetProperty(server *Server, id types.PropertyId) (prop *types.Property, err error) {
	p := PropertyController(types.Property{
		ObjectId:  o.ObjectId,
		ID:        id,
		Index:     0,
		IPAddress: o.IPAddress,
	})

	if err = p.GetValue(server); err != nil {
		return prop, err
	}

	return p.RawValue(), nil
}

func (o ObjectController) RawValue() *types.Object {
	obj := types.Object(o)
	return &obj
}

func (o *ObjectController) GetAllProperties(server *Server) ([]*types.Property, error) {
	if propIds, err := o.GetProperty(server, 371); err != nil {
		return nil, err
	} else {
		properties := make([]*types.Property, 0, len(propIds.Values))

		for _, p := range propIds.Values {
			if prop, err := o.GetProperty(server, p.Value.(uint32)); err != nil {
				continue
			} else {
				properties = append(properties, prop)
			}
		}

		return properties, nil
	}
}
