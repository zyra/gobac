package actions

import (
	"github.com/fatih/color"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet"
	"github.com/zyra/gobac/bacnet/types"
	"time"
)

type ExtendedObject struct {
	*types.Object
	Properties []*types.Property
}

type ExtendedDevice struct {
	*types.Device
	ExtendedObject
	Objects []*ExtendedObject
}

func Scan(ctx *cli.Context) error {
	devices, err := whois(time.Second * 3)

	if err != nil {
		return err
	}

	out := make([]*ExtendedDevice, 0, len(devices))

	for i, d := range devices {
		ed := ExtendedDevice{
			Device: d,
		}

		color.Cyan("| Device [%d/%d] | ID: %d | IP: %s \n", i+1, len(devices), d.ObjectId.Instance, d.IPAddress.String())

		oc := bacnet.ObjectController(d.Object)

		// Get all properties
		if props, err := oc.GetAllProperties(server); err != nil {
			return nil
		} else {
			ed.Properties = props
			for iii, p := range props {
				color.HiYellow("|--| Property <%d/%d> \n", iii+1, len(props))

				for iiii, v := range p.Values {
					color.Blue("|--|-->> Value <%d/%d>: %s \n", iiii+1, len(p.Values), v.ReadAsString())
				}
			}
		}

		// Get all objects
		objs, err := bacnet.DeviceController(*d).GetObjects(server)

		if err != nil {
			return err
		}

		eObjs := make([]*ExtendedObject, len(objs))

		for ii, o := range objs {
			color.Magenta("|--| Object [%d/%d] | Type: %d | ID %d \n", ii+1, len(objs), o.ObjectId.Type, o.ObjectId.Instance)

			c := bacnet.ObjectController(*o)
			if props, err := c.GetAllProperties(server); err != nil {
				return err
			} else {
				eObj := &ExtendedObject{
					Properties: props,
					Object:     o,
				}

				eObjs = append(eObjs, eObj)

				for iii, p := range props {
					color.HiYellow("|--|--| Property <%d/%d> | ID: %d \n", iii+1, p.ID, len(props))

					for iiii, v := range p.Values {
						color.Blue("|--|--|-->> Value <%d/%d>: %s \n", iiii+1, len(p.Values), v.ReadAsString())
					}
				}
			}
		}

		ed.Objects = eObjs

		out = append(out, &ed)
	}

	return nil
}
