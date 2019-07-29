package bacnet

import (
	"fmt"
	"github.com/zyra/bacnet-2/pkg/object"
	"github.com/zyra/bacnet-2/pkg/service"
	_type "github.com/zyra/bacnet-2/pkg/type"
	"testing"
)

var devices *[]*object.Device
var device *object.Device
var objects *[]*object.Object
var ifname = "docker0"
var err error

func TestScan(t *testing.T) {
	devices, err = service.SendWhoIsRequest(ifname)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	dLen := len(*devices)

	if dLen <= 0 {
		t.Error("No devices found")
		t.FailNow()
	}

	device = (*devices)[0]
}

func TestObjects(t *testing.T) {
	if device == nil {
		t.FailNow()
	}

	objects, err = service.ReadProperty(device, _type.PROP_OBJECT_LIST)

	if err != nil {
		fmt.Println("Error!", err)
	}

	fmt.Println(len(*objects))
}
