package bacnet

import (
	"errors"
	"github.com/zyra/gobac/bacnet/types"
)

type DeviceController types.Device

func (d DeviceController) GetObjects(server *Server) (dest []*types.Object, err error) {
	obj := ObjectController(d.Object)

	prop, err := obj.GetProperty(server, types.PropertyObjectList)

	if err != nil {
		return nil, err
	}

	if prop.Values == nil {
		return nil, errors.New("property value is nil")
	}

	dest = prop.ReadValuesAsObjects()

	for _, o := range dest {
		o.IPAddress = d.IPAddress
		o.DeviceID = d.ObjectId.Instance
	}

	return dest, nil
}

func (d DeviceController) RawValue() *types.Device {
	dev := types.Device(d)
	return &dev
}
