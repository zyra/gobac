package session

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// analogValueType is the BACnet object-type code for analog-value objects
// (types.ObjectTypeAnalogValue == 2).
const analogValueType uint16 = uint16(types.ObjectTypeAnalogValue)

// startLiveAgainst starts a Live session configured to reach a simulator
// device listening on 127.0.0.2:port (see simDeviceIP in
// simharness_test.go).
func startLiveAgainst(t *testing.T, port uint16) *Live {
	t.Helper()
	live := NewLive()
	if err := live.Start(Config{Interface: "lo", Port: port, LocalPort: port}); err != nil {
		t.Fatalf("start session: %v", err)
	}
	t.Cleanup(func() { _ = live.Stop() })
	return live
}

func simDeviceAddress() Address {
	return Address{IP: net.ParseIP(simDeviceIP)}
}

func TestReadPropertyAgainstSimulator(t *testing.T) {
	port, shutdown := startSimDevice(t)
	defer shutdown()

	live := startLiveAgainst(t, port)

	obj := ObjectRef{Type: analogValueType, Instance: writableObjectInstance}
	values, err := live.ReadProperty(context.Background(), simDeviceAddress(), obj, uint32(types.PropertyPresentValue))
	if err != nil {
		t.Fatalf("ReadProperty: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected exactly one value, got %d: %#v", len(values), values)
	}
	if values[0].Tag != 4 {
		t.Fatalf("expected tag 4 (Real), got %d", values[0].Tag)
	}
	got, ok := values[0].Value.(float32)
	if !ok || got != float32(42.5) {
		t.Fatalf("expected float32(42.5), got %#v", values[0].Value)
	}
}

func TestWriteThenReadBack(t *testing.T) {
	port, shutdown := startSimDevice(t)
	defer shutdown()

	live := startLiveAgainst(t, port)
	ctx := context.Background()
	obj := ObjectRef{Type: analogValueType, Instance: writableObjectInstance}

	err := live.Write(ctx, simDeviceAddress(), obj, WriteRequest{Tag: 4, Priority: 0, Value: float32(20.25)})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	values, err := live.ReadProperty(ctx, simDeviceAddress(), obj, uint32(types.PropertyPresentValue))
	if err != nil {
		t.Fatalf("ReadProperty after write: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected exactly one value, got %d: %#v", len(values), values)
	}
	got, ok := values[0].Value.(float32)
	if !ok || got != float32(20.25) {
		t.Fatalf("expected float32(20.25), got %#v", values[0].Value)
	}
}

func TestWriteErrorSurfaces(t *testing.T) {
	port, shutdown := startSimDevice(t)
	defer shutdown()

	live := startLiveAgainst(t, port)
	obj := ObjectRef{Type: analogValueType, Instance: readOnlyObjectInstance}

	err := live.Write(context.Background(), simDeviceAddress(), obj, WriteRequest{Tag: 4, Priority: 0, Value: float32(1)})
	if err == nil {
		t.Fatal("expected an error writing to a non-writable property, got nil")
	}
	if !strings.Contains(err.Error(), "WriteAccessDenied") {
		t.Fatalf("expected error to mention WriteAccessDenied, got: %v", err)
	}
}

func TestInstanceGuard(t *testing.T) {
	live := NewLive()
	obj := ObjectRef{Type: analogValueType, Instance: 70000}

	_, err := live.ReadProperty(context.Background(), simDeviceAddress(), obj, uint32(types.PropertyPresentValue))
	if err == nil {
		t.Fatal("expected an error for an out-of-range instance, got nil")
	}
	if !strings.Contains(err.Error(), "22-bit") {
		t.Fatalf("expected error to mention 22-bit, got: %v", err)
	}
}

func TestStartStopLifecycle(t *testing.T) {
	live := NewLive()

	if err := live.Start(Config{Interface: "lo"}); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := live.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := live.Start(Config{Interface: "lo"}); err != nil {
		t.Fatalf("Start after Stop: %v", err)
	}
	defer live.Stop()

	if err := live.Start(Config{Interface: "lo"}); err != ErrAlreadyStarted {
		t.Fatalf("expected ErrAlreadyStarted for a double Start, got: %v", err)
	}
}

func TestReadMultipleCollectsPerPropertyErrors(t *testing.T) {
	port, shutdown := startSimDevice(t)
	defer shutdown()

	live := startLiveAgainst(t, port)
	obj := ObjectRef{Type: analogValueType, Instance: writableObjectInstance}

	specs := []ReadSpec{{
		Object:     obj,
		Properties: []uint32{uint32(types.PropertyPresentValue), 999999},
	}}

	results, err := live.ReadMultiple(context.Background(), simDeviceAddress(), specs)
	if err != nil {
		t.Fatalf("ReadMultiple: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly one ObjectResult, got %d", len(results))
	}

	result := results[0]
	if len(result.Values) != 1 {
		t.Fatalf("expected exactly one value, got %d: %#v", len(result.Values), result.Values)
	}
	got, ok := result.Values[0].Value.(float32)
	if !ok || got != float32(42.5) {
		t.Fatalf("expected float32(42.5), got %#v", result.Values[0].Value)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %d: %#v", len(result.Errors), result.Errors)
	}
	if result.Errors[999999] == nil {
		t.Fatal("expected an error for unknown property 999999")
	}
}
