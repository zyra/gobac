package gobac

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

var server *Server
var devices = make([]*Device, 0)
var device *Device
var objects = make([]*Object, 0)
var isBench = os.Getenv("BENCH") != ""
var ifname = "docker0"
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
		s := &stats{0,0}

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
