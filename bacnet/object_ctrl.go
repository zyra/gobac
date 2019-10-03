package bacnet

import (
	context2 "context"
	"github.com/zyra/gobac/bacnet/types"
	"strings"
)

type ObjectController types.Object

func (o *ObjectController) GetProperty(server Server, id types.PropertyId) (prop *types.Property, err error) {
	p := PropertyController(types.Property{
		ObjectId:  o.ObjectId,
		ID:        id,
		Index:     0,
		IPAddress: o.IPAddress,
	})

	if err = p.GetValue(context2.TODO(), server); err != nil {
		return prop, err
	}

	return p.RawValue(), nil
}

func (o ObjectController) RawValue() *types.Object {
	obj := types.Object(o)
	return &obj
}

func (o *ObjectController) GetAllProperties(server Server) ([]*types.Property, error) {
	propIds := make([]types.PropertyId, 0)

	propIdsProp, err := o.GetProperty(server, 371)

	if err != nil {
		if strings.Contains(err.Error(), "ErrorCodeUnknownProperty") {
			// Device doesn't support property 371
			// let's check for known properties
			propIds = []types.PropertyId{
				types.PropertyObjectName,
				types.PropertyObjectList,
				types.PropertyDescription,
				types.PropertyPresentValue,
				types.PropertyModelName,
				types.PropertyVendorName,
				types.PropertyVendorIdentifier,
				types.PropertySystemStatus,
				types.PropertyLocation,
				types.PropertyFirmwareRevision,
				types.PropertyApplicationSoftwareVersion,
				types.PropertyStateText,
			}
		} else {
			return nil, err
		}
	} else if propIdsProp == nil || propIdsProp.Values == nil {
		return nil, err
	} else {
		hasName := false

		for _, v := range propIdsProp.Values {
			if vv, ok := v.Value.(uint32); ok {
				hasName = hasName || vv == types.PropertyObjectName
				propIds = append(propIds, vv)
			}
		}

		if !hasName {
			propIds = append(propIds, types.PropertyObjectName)
		}
	}

	properties := make([]*types.Property, 0, len(propIds))

	for _, p := range propIds {
		if prop, err := o.GetProperty(server, p); err != nil {
			continue
		} else {
			properties = append(properties, prop)
		}
	}

	return properties, nil
}

func (o *ObjectController) GetPresentValue(server Server) (*types.PropertyValue, error) {
	p, e := o.GetProperty(server, types.PropertyPresentValue)

	if e != nil || p == nil || p.Values == nil || len(p.Values) == 0 {
		return nil, e
	}

	return p.Values[0], nil
}
