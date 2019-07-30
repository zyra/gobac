package gobac

import (
	"fmt"
	"testing"
)

var server *Server
var devices = make([]*Device, 0)
var device *Device
var objects = make([]*Object, 0)
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

	device = devices[0]
}

func TestObjects(t *testing.T) {
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
