package session

import (
	"context"
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/responder"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/simulator"
)

// simDeviceIP is the loopback address the in-process simulator device binds
// to. It deliberately isn't 127.0.0.1: this library's client always sends
// requests from the exact port it listens on (see bacnet.Request.Send),
// so a Live session bound to the "lo" interface ends up sending from
// 127.0.0.1:<port>. If the simulator device also bound 127.0.0.1:<port>,
// the two would collide on the identical socket address and the device's
// reply would loop back to itself instead of reaching the client. Binding
// the device to a distinct loopback address avoids that, while staying
// entirely within 127.0.0.0/8 on an ephemeral port.
const simDeviceIP = "127.0.0.2"

// writableObjectInstance and readOnlyObjectInstance are the analog-value
// instances startSimDevice registers on the simulated device.
const (
	writableObjectInstance uint32 = 1
	readOnlyObjectInstance uint32 = 2
)

// startSimDevice builds a minimal one-device scenario in code and serves it
// over loopback UDP in a background goroutine. It returns the ephemeral
// port the device is listening on and a shutdown func the caller must
// invoke to stop the goroutine and release the socket.
func startSimDevice(t *testing.T) (port uint16, shutdown func()) {
	t.Helper()

	scenario := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{
			ID:   1001,
			Name: "test-dev",
			Objects: []simulator.ObjectSpec{
				{
					Type:         "analog-value",
					Instance:     writableObjectInstance,
					Name:         "writable-av",
					PresentValue: 42.5,
					Writable:     true,
				},
				{
					Type:         "analog-value",
					Instance:     readOnlyObjectInstance,
					Name:         "read-only-av",
					PresentValue: 10.0,
					Writable:     false,
				},
			},
		}},
	}

	if err := scenario.Validate(); err != nil {
		t.Fatalf("invalid scenario: %v", err)
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		t.Fatalf("build network: %v", err)
	}
	device, err := network.Device(1001)
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	conn, err := transport.ListenUDP(transport.NewEndpoint(net.ParseIP(simDeviceIP), 0))
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}

	respServer := responder.NewServer()
	simulator.NewApplication(device, simulator.RealClock{}).Register(respServer)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			datagram, err := conn.Read(ctx)
			if err != nil {
				return
			}
			_ = respServer.ServeDatagram(ctx, conn, datagram)
		}
	}()

	shutdown = func() {
		cancel()
		_ = conn.Close()
		<-done
	}

	return uint16(conn.LocalEndpoint().Port), shutdown
}
