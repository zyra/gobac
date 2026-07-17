//go:build aix || darwin || dragonfly || netbsd || openbsd
// +build aix darwin dragonfly netbsd openbsd

package bacnet

import (
	"context"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func TestListenBroadcastUDPSetsSoReusePort(t *testing.T) {
	conn, err := listenBroadcastUDP(context.Background(), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listenBroadcastUDP: %v", err)
	}
	defer conn.Close()

	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}

	var value int
	var getErr error
	if ctrlErr := raw.Control(func(fd uintptr) {
		value, getErr = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT)
	}); ctrlErr != nil {
		t.Fatalf("raw.Control: %v", ctrlErr)
	}
	if getErr != nil {
		t.Fatalf("GetsockoptInt: %v", getErr)
	}
	if value == 0 {
		t.Fatal("SO_REUSEPORT is not set on the generic listener")
	}
}
