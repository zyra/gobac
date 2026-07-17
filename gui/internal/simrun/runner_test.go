package simrun

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/zyra/gobac/gui/assets"
	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/v2/bacnet/types"
	"github.com/zyra/gobac/v2/simulator"
)

// analogInputType and analogValueType are the BACnet object-type codes used
// against the embedded quickstart scenario's devices.
const (
	analogInputType uint16 = uint16(types.ObjectTypeAnalogInput)
	analogValueType uint16 = uint16(types.ObjectTypeAnalogValue)
)

func decodeQuickstartScenario(t *testing.T) *simulator.Scenario {
	t.Helper()
	sc, err := simulator.DecodeScenario(bytes.NewReader(assets.QuickstartScenario), "yaml")
	if err != nil {
		t.Fatalf("DecodeScenario: %v", err)
	}
	return sc
}

func findDevice(t *testing.T, devices []RunningDevice, id uint32) RunningDevice {
	t.Helper()
	for _, d := range devices {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("no running device with id %d in %+v", id, devices)
	return RunningDevice{}
}

// loopbackInterfaceName returns the platform-specific name of the up
// loopback interface with an IPv4 address, mirroring
// internal/session/live_test.go's helper of the same purpose (unexported
// there, so duplicated here rather than shared across package boundaries).
func loopbackInterfaceName(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("list network interfaces: %v", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback == 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				return iface.Name
			}
		}
	}
	t.Fatal("no up loopback interface with an IPv4 address found")
	return ""
}

// clientAgainst starts a session.Live configured to reach a device on
// 127.0.0.2:port (see deviceIP in runner.go).
func clientAgainst(t *testing.T, port uint16) *session.Live {
	t.Helper()
	live := session.NewLive()
	cfg := session.Config{Interface: loopbackInterfaceName(t), Port: port, LocalPort: port}
	if err := live.Start(cfg); err != nil {
		t.Fatalf("start session: %v", err)
	}
	t.Cleanup(func() { _ = live.Stop() })
	return live
}

func TestRunnerServesQuickstartDevicesOverLoopback(t *testing.T) {
	sc := decodeQuickstartScenario(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, err := Start(ctx, sc)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer runner.Stop()

	devices := runner.Devices()
	if len(devices) != 3 {
		t.Fatalf("len(Devices()) = %d, want 3", len(devices))
	}

	wantIDs := map[uint32]bool{1001: true, 1002: true, 1003: true}
	seenPorts := make(map[uint16]int)
	for _, d := range devices {
		if !wantIDs[d.ID] {
			t.Errorf("unexpected running device id %d", d.ID)
		}
		delete(wantIDs, d.ID)
		if d.Port == 0 {
			t.Errorf("device %d has a zero port", d.ID)
		}
		seenPorts[d.Port]++
	}
	if len(wantIDs) != 0 {
		t.Errorf("missing running devices for ids %v", wantIDs)
	}
	for port, count := range seenPorts {
		if count != 1 {
			t.Errorf("port %d used by %d devices, want distinct ports", port, count)
		}
	}

	dev1001 := findDevice(t, devices, 1001)
	live := clientAgainst(t, dev1001.Port)
	addr := session.Address{IP: net.ParseIP(dev1001.Addr)}

	// analog-input(0) instance 1 "Supply Temp" present_value 72.5.
	values, err := live.ReadProperty(context.Background(), addr,
		session.ObjectRef{Type: analogInputType, Instance: 1}, uint32(types.PropertyPresentValue))
	if err != nil {
		t.Fatalf("ReadProperty(Supply Temp): %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected exactly one value, got %d: %#v", len(values), values)
	}
	if got, ok := values[0].Value.(float32); !ok || got != float32(72.5) {
		t.Fatalf("Supply Temp = %#v, want float32(72.5)", values[0].Value)
	}

	// analog-value(2) instance 2 "Setpoint": write then read back.
	setpoint := session.ObjectRef{Type: analogValueType, Instance: 2}
	writeErr := live.Write(context.Background(), addr, setpoint,
		session.WriteRequest{Tag: 4, Priority: 8, Value: float32(69.0)})
	if writeErr != nil {
		t.Fatalf("Write(Setpoint): %v", writeErr)
	}
	values, err = live.ReadProperty(context.Background(), addr, setpoint, uint32(types.PropertyPresentValue))
	if err != nil {
		t.Fatalf("ReadProperty(Setpoint) after write: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected exactly one value, got %d: %#v", len(values), values)
	}
	if got, ok := values[0].Value.(float32); !ok || got != float32(69.0) {
		t.Fatalf("Setpoint after write = %#v, want float32(69.0)", values[0].Value)
	}

	runner.Stop()

	readCtx, readCancel := context.WithTimeout(context.Background(), time.Second)
	defer readCancel()
	if _, err := live.ReadProperty(readCtx, addr,
		session.ObjectRef{Type: analogInputType, Instance: 1}, uint32(types.PropertyPresentValue)); err == nil {
		t.Fatal("expected a read after Stop to fail, got nil error")
	}
}

func TestStartRejectsMultiIPScenario(t *testing.T) {
	sc := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "multi-ip"},
		Devices: []simulator.DeviceSpec{
			{ID: 1, Name: "dev-a", Address: "127.0.0.1", Port: 47901},
			{ID: 2, Name: "dev-b", Address: "127.0.0.1", Port: 47902},
		},
	}

	_, err := Start(context.Background(), sc)
	if err == nil {
		t.Fatal("expected an error for a multi-ip scenario, got nil")
	}
	if !strings.Contains(err.Error(), "quickstart requires multi-port loopback scenarios") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "quickstart requires multi-port loopback scenarios")
	}
}
