package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/types"
)

type ExtendedObject struct {
	*types.Object
	Properties  []*types.Property
	FloatValue  float64
	StringValue string
}

type ExtendedDevice struct {
	*types.Device
	Properties []*types.Property
	Objects    []*ExtendedObject
}

func Scan(ctx *cli.Context) error {
	devices, err := whois(time.Second * 3)

	if err != nil {
		return err
	}

	//isJson := ctx.GlobalBool("json")

	out := make([]*ExtendedDevice, len(devices), len(devices))

	wg := sync.WaitGroup{}
	wg.Add(len(devices))

	for i, d := range devices {
		go func(i int, d *types.Device) {
			defer wg.Done()

			ed := ExtendedDevice{
				Device: d,
			}

			//if !isJson {
			//	color.Cyan("| Device [%d/%d] | ID: %d | IP: %s \n", i+1, len(devices), d.ObjectId.Instance, d.IPAddress.String())
			//}

			oc := bacnet.ObjectController(d.Object)

			// Get all properties
			if props, err := oc.GetAllProperties(server); err != nil {
				log.Fatal(err)
			} else {
				ed.Properties = props

				for _, p := range props {
					switch p.ID {
					case types.PropertyObjectName:
						ed.Name = p.Values[0].ReadAsString()

						break

					case types.PropertyDescription:
						ed.Description = p.Values[0].ReadAsString()
						break
					}
				}

				//if !isJson {
				//	for iii, p := range props {
				//		color.HiYellow("|--| Property <%d/%d> | ID: %d \n", iii+1, len(props), p.ID)
				//
				//		for iiii, v := range p.Values {
				//			color.Blue("|--|-->> Value <%d/%d>: %s \n", iiii+1, len(p.Values), v.ReadAsString())
				//		}
				//	}
				//}

			}

			// Get all objects
			objs, err := bacnet.DeviceController(*d).GetObjects(server)

			if err != nil {
				println("Error getting objects", err.Error())
				log.Fatal(err)
			}

			eObjs := make([]*ExtendedObject, len(objs))

			objsWg := sync.WaitGroup{}
			objsWg.Add(len(objs))

			for ii, o := range objs {
				//if !isJson {
				//	color.Magenta("|--| Object [%d/%d] | Type: %d | ID %d \n", ii+1, len(objs), o.ObjectId.Type, o.ObjectId.Instance)
				//}

				go func(ii int, o *types.Object) {
					defer objsWg.Done()

					c := bacnet.ObjectController(*o)
					if props, err := c.GetAllProperties(server); err != nil {
						println("Error getting propss", err.Error())
						log.Fatal(err)
					} else {
						eObj := &ExtendedObject{
							Properties: props,
							Object:     o,
						}

						for _, p := range props {
							switch p.ID {
							case types.PropertyObjectName:
								eObj.Name = p.Values[0].ReadAsString()
								break

							case types.PropertyDescription:
								eObj.Description = p.Values[0].ReadAsString()
								break

							case types.PropertyPresentValue:
								eObj.PresentValue = p.Values[0]
								eObj.FloatValue = p.Values[0].ReadAsFloat64()
								eObj.StringValue = p.Values[0].ReadAsString()
								break
							}
						}

						eObjs[ii] = eObj

						//if !isJson {
						//	for iii, p := range props {
						//		color.HiYellow("|--|--| Property <%d/%d> | ID: %d \n", iii+1, len(props), p.ID)
						//
						//		for iiii, v := range p.Values {
						//			color.Blue("|--|--|-->> Value <%d/%d>: %s \n", iiii+1, len(p.Values), v.ReadAsString())
						//		}
						//	}
						//}

					}
				}(ii, o)
			}

			objsWg.Wait()

			ed.Objects = eObjs
			out[i] = &ed
		}(i, d)
	}

	wg.Wait()

	if j, e := json.Marshal(out); e != nil {
		log.Fatal("Error marshasling to json", e)
	} else {
		fmt.Println(string(j))
	}

	return nil
}
