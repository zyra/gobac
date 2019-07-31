package gobac

import (
	"fmt"
	"github.com/zyra/gobac/types"
	"os"
	"sync"
	"testing"
)

var server *Server
var devices = make([]*Device, 0)
var device *Device
var objects = make([]*Object, 0)
var isBench = os.Getenv("BENCH") != ""
var ifname = "eno1.5"
var err error

func TestNewServer(t *testing.T) {
	server, err = NewServer(ifname)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if server.InterfaceName != ifname {
		t.Errorf("expected interface name to be %s and got %s\n", ifname, server.InterfaceName)
		t.FailNow()
	}

	server.Listen()
}

func TestScan(t *testing.T) {
	err = server.WhoIs(&devices)

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

	//for _, d := range devices {
	//fmt.Printf("> Device ID: %d\n", d.Instance)
	//}

	device = devices[0]
}

type stats struct {
	s int
	f int
}

func TestObjects(t *testing.T) {
	if isBench {
		twg := &sync.WaitGroup{}
		s := &stats{0, 0}

		fmt.Printf("going to get objects from %d devices\n", len(devices))

		for i := 0; i < len(devices); i++ {
			twg.Add(1)
			go func(wg *sync.WaitGroup, device *Device, s *stats) {
				defer wg.Done()
				objects := make([]*Object, 0)
				if err := device.GetObjects(&objects); err != nil {
					fmt.Printf("error getting objects: %s\n", err)
					s.f++
				} else {
					s.s++
				}
			}(twg, devices[i], s)
		}

		twg.Wait()

		fmt.Printf("Total success: %d\nTotal failure: %d\n", s.s, s.f)

		return
	}

	if device == nil {
		t.FailNow()
	}

	if err = device.GetObjects(&objects); err != nil {
		t.Error(err)
		t.FailNow()
	}

	olen := len(objects)

	if olen <= 0 {
		t.Error("No objects found")
		t.FailNow()
	}

	fmt.Println(len(objects))
}

func TestWrite(t *testing.T) {
	if len(objects) == 0 {
		t.Error("objects array has length of 0")
		t.FailNow()
	}

	var obj *Object

	for _, o := range objects {
		if o.Type == types.OBJECT_ANALOG_VALUE {
			obj = o
			break
		}
	}

	if obj == nil {
		t.Error("couldn't find an AO obj")
		t.FailNow()
	}

	prop, err := obj.GetProperty(types.PROP_PRESENT_VALUE)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if err := prop.SetValue(TagReal, 3); err != nil {
		t.Error(err)
	} else {
		fmt.Println("Wrote prop to obj!")
		prop, err = obj.GetProperty(types.PROP_PRESENT_VALUE)

		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		fmt.Println("Read the same prop again")
	}
}

func TestServer_SendCovRequest(t *testing.T) {
	if len(objects) == 0 {
		t.Error("objects array has length of 0")
		t.FailNow()
	}

	var obj *Object

	for _, o := range objects {
		if o.Type == types.PROP_ACTION_TEXT {
			obj = o
			break
		}
	}

	if obj == nil {
		t.Error("couldn't find an AO obj")
		t.FailNow()
	}

	prop, err := obj.GetProperty(types.PROP_PRESENT_VALUE)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	data := make(chan *Property, 5000)
	e := make(chan error, 5000)
	done := make(chan int, 5000)

	server.SendCovRequest(prop.Object, 50, data, e, done)

	//if err := prop.SetValue(TagReal, 2); err != nil {
	//	t.Error(err)
	//} else {
		select {
		case prop := <-data:
			fmt.Println("Got a property!", prop)
			if prop.Values == nil || len(*prop.Values) == 0 {
				t.Error("values are empty or null")
				t.FailNow()
			}

			if (*prop.Values)[0].Value.(float32) != 2 {
				t.Errorf("expected value to be %d, got %f", 2, (*prop.Values)[0].Value.(float32))
				t.FailNow()
			}

			fmt.Println("all good!")
			break

		case err = <-e:
			t.Error(err)
			t.FailNow()

		case _ = <-done:
			fmt.Println("received done signal?")
			break
		}
	//}
}
