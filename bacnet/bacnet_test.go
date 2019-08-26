package bacnet

import (
	"context"
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
	"testing"
	"time"
)

var config *ServerConfig
var server *Server
var devices = make([]*types.Device, 0)
var device *types.Device
var devCtrl *DeviceController
var objects = make([]*types.Object, 0)
var ifname = "docker0"
var err error
var ctx context.Context

func configureServer() {
	config = NewServerConfig().SetInterfaceName(ifname)
	config.SetDefaultTimeout(time.Second * 3)
}

func createTestServer() error {
	if config == nil {
		configureServer()
	}

	server, err = NewServer(config)

	if err != nil {
		return err
	}

	if server.InterfaceName != ifname {
		return fmt.Errorf("expected interface name to be %s and got %s\n", ifname, server.InterfaceName)
	}

	fmt.Println("Starting Server..")
	go server.Listen(ctx)

	<-server.Start()
	fmt.Println("Server started")

	return nil
}

func TestNewServer(t *testing.T) {
	configureServer()
	if err := createTestServer(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkNewServer(b *testing.B) {
	c, cc := context.WithCancel(context.Background())
	ctx = c

	if err := createTestServer(); err != nil {
		b.Fatal(err)
	}

	cc()

	<-server.close
}

var count = 0

func testScan() error {
	dChan, err := server.WhoIs(time.Millisecond * 250)

	if err != nil {
		return err
	}

	for {
		dev, open := <-dChan

		if dev != nil {
			devices = append(devices, dev)
		}

		if !open {
			break
		}
		count++
	}

	//dLen := len(devices)
	//
	//if dLen <= 0 {
	//	return errors.New("no devices found")
	//}
	//
	//fmt.Printf("Found %d devices\n", dLen)

	//for _, d := range devices {
	//	fmt.Printf("> Device ID: %d\n", d.ObjectId.Instance)
	//}

	return nil
}

func TestScan(t *testing.T) {
	if err := testScan(); err != nil {
		t.Fatal(err)
	}

	if len(devices) == 0 {
		t.Fatal("no devices found")
	}

	device = devices[0]
	ctrl := DeviceController(*device)
	devCtrl = &ctrl
}

func BenchmarkScan(b *testing.B) {
	c, cc := context.WithCancel(context.Background())

	ctx = c

	configureServer()

	if err := createTestServer(); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		if err := testScan(); err != nil {
			b.Fatal(err)
		}
	}

	fmt.Println(count)

	cc()

	<-server.close
}

func TestObjects(t *testing.T) {
	if len(devices) < 1 {
		t.FailNow()
	}

	if objs, err := devCtrl.GetObjects(server); err != nil {
		fmt.Printf("error getting objects: %s\n", err)
		t.Fatal(err)
	} else {
		objects = objs
	}

	olen := len(objects)

	if olen <= 0 {
		t.Fatal("No objects found")
	}

	fmt.Println(len(objects))
}

func TestWrite(t *testing.T) {
	var obj *types.Object

	for _, o := range objects {
		if o.ObjectId.Type == types.ObjectTypeAnalogValue {
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
	for i, o := range objects {
		if i == 0 {
			// skip device
			continue
		}

		fmt.Printf("> processing %d out of %d\n", i, len(objects))

		objCtrl := ObjectController(*o)

		if p, err := objCtrl.GetProperty(server, types.PropertyObjectName); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("name is ", p.Values[0].ReadAsString())
		}

		if p, err := objCtrl.GetProperty(server, types.PropertyStateText); err == nil && p != nil {
			fmt.Println(p.Values[0].ReadAsString(), len(p.Values))
		}

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
	return

	var obj *types.Object

	for _, o := range objects {
		if ObjectController(*o).ObjectId.Type == types.ObjectTypeAnalogValue {
			obj = o
			break
		}
	}

	if obj == nil {
		t.Error("couldn't find an MSV obj")
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

	if err := propCtrl.SetValue(server, TagReal, 2); err != nil {
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
					if p.Values != nil && len(p.Values) > 0 && p.Values[0].Type == types.TagReal {
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

				if prop.Value.(types.Real) != 2 {
					t.Errorf("expected value to be %d, got %f", 2, prop.Value.(types.Real))
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
