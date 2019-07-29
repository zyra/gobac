package bacnet

import (
	"fmt"
	"net"
	"testing"
)

func TestMac(t *testing.T) {
	iface, err := net.InterfaceByName("docker0")

	if err != nil {
		t.FailNow()
	}

	fmt.Println(iface.HardwareAddr)
}

func TestScan(t *testing.T) {
	res := Scan()

	if res == nil {
		t.FailNow()
	}
}
