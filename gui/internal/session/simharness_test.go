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
// to. It deliberately isn't 127.0.0.1: this library always sends requests
// to the destination port equal to the client's own configured broadcast
// port (bacnet/request.go:160, server.GetBroadcastPort()), and on Linux the
// client's single unicast/broadcast socket binds the wildcard address at
// that same port (bacnet/server_posix.go:63). Since the tests must set that
// port to the simulator device's actual (ephemeral) port for requests to
// reach it, the client's effective loopback source address becomes
// 127.0.0.1:<port> — identical to what the device would bind if it also
// used 127.0.0.1:<port>. That collision would make replies loop back to the
// device's own socket instead of reaching the client. Binding the device to
// a distinct loopback address avoids that, while staying entirely within
// 127.0.0.0/8 on an ephemeral port.
//
// This is a deviation from plan §6.7's original text ("UDP-using tests bind
// only 127.0.0.1"), forced by the library's shared-port addressing model
// rather than chosen for convenience — see gui-architecture.md §6.7 and its
// amendment in §10.A (owner-approved 2026-07-17: loopback addresses beyond
// 127.0.0.1 are allowed provided every CI OS leg that runs the test
// provisions the alias). gui-validate.yml carries alias-provisioning steps
// for macOS (`ifconfig lo0 alias`) and Windows (`netsh interface ipv4 add
// address`) to keep this address reachable on those CI legs; Linux routes
// all of 127.0.0.0/8 to loopback natively and needs no step.
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
