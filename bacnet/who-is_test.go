package bacnet

import (
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
	"testing"
	"time"
)

func TestServer_WhoIs(t *testing.T) {
	TestNewServer(t)
	dChan, err := server.WhoIs(time.Second * 2)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	ids := make([]types.Uint16, 0)

	for {
		dev, open := <-dChan

		if dev != nil {
			fmt.Printf("> Device ID: %d\n", dev.ObjectId.Instance)
			ids = append(ids, dev.ObjectId.Instance)
		}

		if !open {
			break
		}
	}

	if len(ids) == 0 {
		t.Error("No devices found")
		t.FailNow()
	}

	for i, id := range ids {
		for i2, id2 := range ids {
			if id == id2 && i != i2 {
				t.Errorf("Device ID found twice: %d", id)
			}
		}
	}
}
