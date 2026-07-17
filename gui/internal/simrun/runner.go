// Package simrun runs a simulator.Scenario's devices in-process, each as a
// loopback UDP BACnet/IP responder, for the Quickstart view (task G8). It
// has no dependency on Fyne and is unit-tested on its own.
package simrun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/zyra/gobac/v2/bacnet/responder"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/simulator"
)

// errUnsupportedScenario is returned by Start when sc is not a loopback
// multi-port (or single-device) scenario.
var errUnsupportedScenario = errors.New("quickstart requires multi-port loopback scenarios")

// deviceIP is the loopback address every in-process device binds to. It
// deliberately is not 127.0.0.1: the wrapped client library always sends a
// request to the destination port equal to its own configured listen port
// (bacnet/request.go:160), so reading/writing a given device requires a
// session.Live configured with that device's port — at which point the
// client's own loopback socket and a 127.0.0.1-bound device would occupy
// the same address:port pair, and replies would loop back to the client's
// own socket instead of reaching it. A distinct loopback address avoids
// that collision while staying inside 127.0.0.0/8 (gui-architecture.md
// §10.A); see internal/session's simDeviceIP for the fuller explanation and
// the CI provisioning this requires, already carried by gui-validate.yml
// for every OS leg.
const deviceIP = "127.0.0.2"

// RunningDevice describes one device the Runner has brought up.
type RunningDevice struct {
	ID   uint32
	Name string
	Addr string
	Port uint16
}

// Runner runs every device of a scenario as an in-process BACnet/IP
// responder over loopback UDP.
type Runner struct {
	devices []RunningDevice
	conns   []transport.Conn

	cancel context.CancelFunc
	wg     sync.WaitGroup

	errCh    chan error
	stopOnce sync.Once
}

// Start validates sc, builds its object model, and brings up one loopback
// UDP responder per device, each serving on a goroutine. It requires a
// loopback multi-port (or single-device) scenario with no non-loopback
// device addresses: quickstart runs are scoped to loopback so they can
// never collide with, or be mistaken for, a real BACnet/IP network
// (gui-architecture.md §4.5).
//
// A device whose DeviceSpec.Port is 0 is bound to an OS-assigned ephemeral
// port; the real port is reported in the corresponding RunningDevice.
func Start(ctx context.Context, sc *simulator.Scenario) (*Runner, error) {
	if sc == nil {
		return nil, errors.New("scenario is nil")
	}
	if sc.Network.Mode != "multi-port" && sc.Network.Mode != "single-device" {
		return nil, errUnsupportedScenario
	}
	for i := range sc.Devices {
		if !isLoopbackOrEmpty(sc.Devices[i].Address) {
			return nil, errUnsupportedScenario
		}
	}

	if err := sc.Validate(); err != nil {
		return nil, err
	}
	network, err := sc.BuildNetwork()
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	r := &Runner{
		cancel: cancel,
		errCh:  make(chan error, len(sc.Devices)),
	}

	for _, spec := range sc.Devices {
		device, err := network.Device(spec.ID)
		if err != nil {
			r.Stop()
			return nil, fmt.Errorf("device %d: %v", spec.ID, err)
		}

		endpoint := transport.NewEndpoint(net.ParseIP(deviceIP), spec.Port)
		conn, err := transport.ListenUDP(endpoint)
		if err != nil {
			r.Stop()
			return nil, fmt.Errorf("listen for device %d: %v", spec.ID, err)
		}
		r.conns = append(r.conns, conn)

		actual := conn.LocalEndpoint()
		r.devices = append(r.devices, RunningDevice{
			ID:   spec.ID,
			Name: spec.Name,
			Addr: actual.IP.String(),
			Port: actual.Port,
		})

		respServer := responder.NewServer()
		simulator.NewApplication(device, simulator.RealClock{}).Register(respServer)

		r.wg.Add(1)
		go r.serve(runCtx, respServer, conn)
	}

	return r, nil
}

// serve runs one device's receive loop until conn is closed or ctx is
// canceled (both happen together, from Stop). Errors from an unrequested
// shutdown are forwarded on errCh; errors caused by Stop are expected and
// dropped.
func (r *Runner) serve(ctx context.Context, respServer *responder.Server, conn transport.Conn) {
	defer r.wg.Done()
	err := respServer.Serve(ctx, conn)
	if err != nil && ctx.Err() == nil {
		select {
		case r.errCh <- err:
		default:
		}
	}
}

// isLoopbackOrEmpty reports whether addr is empty or parses as a loopback
// IP address.
func isLoopbackOrEmpty(addr string) bool {
	if addr == "" {
		return true
	}
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}

// Devices returns the devices Start brought up, in scenario order.
func (r *Runner) Devices() []RunningDevice {
	out := make([]RunningDevice, len(r.devices))
	copy(out, r.devices)
	return out
}

// Stop shuts every device's listener down and waits for all serve
// goroutines to exit. It is idempotent and safe to call even if Start
// failed partway through.
func (r *Runner) Stop() {
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		for _, conn := range r.conns {
			_ = conn.Close()
		}
	})
	r.wg.Wait()
}

// Err returns a channel of fatal per-device serve errors — failures not
// caused by Stop. It is never closed.
func (r *Runner) Err() <-chan error {
	return r.errCh
}
