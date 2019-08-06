package bacnet

import (
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
	"os"
	"testing"
	"time"
)

var server *Server
var devices = make([]*types.Device, 0)
var device *types.Device
var devCtrl *DeviceController
var objects = make([]*types.Object, 0)
var isBench = os.Getenv("BENCH") != ""
var ifname = "docker0"
var err error

func TestNewServer(t *testing.T) {
	config := NewServerConfig().SetInterfaceName(ifname)
	config.SetDefaultTimeout(time.Second * 3)
	server, err = NewServer(config)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if server.InterfaceName != ifname {
		t.Errorf("expected interface name to be %s and got %s\n", ifname, server.InterfaceName)
		t.FailNow()
	}

	fmt.Println("Starting Server..")
	go server.Listen(nil)

	<-server.Start()
	fmt.Println("Server started")
}

func TestScan(t *testing.T) {
	devices, err = server.WhoIs(time.Millisecond * 500)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	dLen := len(devices)

	if dLen <= 0 {
		t.Error("No devices found")
		t.FailNow()
	}

	fmt.Printf("Found %d devices\n", dLen)

	for _, d := range devices {
		fmt.Printf("> Device ID: %d\n", d.ObjectId.Instance)
	}

	device = devices[0]
	ctrl := DeviceController(*device)
	devCtrl = &ctrl
}

type stats struct {
	s int
	f int
}

func TestObjects(t *testing.T) {
	if len(devices) < 1 {
		t.FailNow()
	}

	if isBench {
		//twg := &sync.WaitGroup{}
		s := &stats{0, 0}

		fmt.Printf("going to get objects from %d devices\n", len(devices))

		//for i := 0; i < len(devices); i++ {
		//twg.Add(1)
		//go func(wg *sync.WaitGroup, device *Device, s *stats) {
		//	defer wg.Done()
		if objs, err := devCtrl.GetObjects(server); err != nil {
			fmt.Printf("error getting objects: %s\n", err)
			s.f++
		} else {
			s.s++
			objects = objs
		}
		//}(twg, devices[i], s)
		//}

		//twg.Wait()

		fmt.Printf("Total success: %d\nTotal failure: %d\n", s.s, s.f)
	}

	if objs, err := devCtrl.GetObjects(server); err != nil {
		fmt.Printf("error getting objects: %s\n", err)
		t.Error(err)
		t.FailNow()
	} else {
		objects = objs
	}

	olen := len(objects)

	if olen <= 0 {
		t.Error("No objects found")
		t.FailNow()
	}

	fmt.Println(len(objects))
}

func TestWrite(t *testing.T) {
	//return
	if len(objects) == 0 {
		t.Error("objects array has length of 0")
		t.FailNow()
	}

	var obj *types.Object

	for _, o := range objects {
		if o.ObjectId.Type == types.ObjectTypeAnalogValue {
			obj = o
			break
		}
	}

	if &obj == nil {
		t.Error("couldn't find an AO obj")
		t.FailNow()
	}

	objCtrl := ObjectController(*obj)

	prop, err := objCtrl.GetProperty(server, types.PropertyPresentValue)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	propCtrl := PropertyController(*prop)

	if err := propCtrl.SetValue(server, TagReal, 1); err != nil {
		t.Error(err)
	} else {
		fmt.Println("Wrote prop to obj!")
		prop, err = objCtrl.GetProperty(server, types.PropertyPresentValue)

		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		if prop.Values[0].Value != types.Real(1) {
			t.Error("value didnt change")
			t.FailNow()
		}
		fmt.Println("Read the same prop again")
	}
}

func TestReadAll(t *testing.T) {
	if len(objects) < 1 {
		t.FailNow()
	}

	//return
	for i, o := range objects {
		if i == 0 {
			// skip device
			continue
		}

		fmt.Printf("> processing %d out of %d\n", i, len(objects))

		objCtrl := ObjectController(*o)

		if _, err := objCtrl.GetProperty(server, types.PropertyPresentValue); err != nil {
			fmt.Println(err)
		}

		if p, err := objCtrl.GetProperty(server, types.PropertyDescription); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("> Description is %s\n", p.Values[0].Value)
		}
	}
}

func TestServer_SendCovRequest(t *testing.T) {
	if len(objects) < 1 {
		t.Error("objects array has length of 0")
		t.FailNow()
	}

	var obj *types.Object

	for _, o := range objects {
		if ObjectController(*o).ObjectId.Type == types.ObjectTypeMultiStateValue {
			obj = o
			break
		}
	}

	if obj == nil {
		t.Error("couldn't find an AO obj")
		t.FailNow()
	}

	objCtrl := ObjectController(*obj)

	prop, err := objCtrl.GetProperty(server, types.PropertyPresentValue)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	propCtrl := PropertyController(*prop)

	req, err := server.SubscribeCov(obj.IPAddress, obj.ObjectId.Type, obj.ObjectId.Instance, 2, false)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	fmt.Println("sent cov req")

	if err := propCtrl.SetValue(server, TagUnsigned, 2); err != nil {
		t.Error(err)
	} else {

		fmt.Println("Value was set to 2")

	Loop:
		for {
			select {
			case <-time.After(time.Second * 3):
				t.Error("didn't get notification within 3 seconds")
				t.FailNow()
			case n := <-req.Data():
				if n.ObjectId.Type != objCtrl.ObjectId.Type || n.ObjectId.Instance != objCtrl.ObjectId.Instance {
					continue
				}

				var prop *types.PropertyValue

				for _, p := range n.Properties {
					if p.Values != nil && len(p.Values) > 0 && p.Values[0].Type == types.TagEnumerated {
						prop = p.Values[0]
						break
					}
				}

				if prop == nil {
					continue
				}

				fmt.Println("Got a property!", prop)
				if prop.Value == nil {
					t.Error("value is null")
					t.FailNow()
				}

				if prop.Value.(uint32) != 2 {
					t.Errorf("expected value to be %d, got %d", 2, prop.Value.(uint32))
					t.FailNow()
				}

				fmt.Println("all good!")
				break Loop

			case err = <-req.Error():
				t.Error(err)
				t.FailNow()
			}
		}
	}
}
