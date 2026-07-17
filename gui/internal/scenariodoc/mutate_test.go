package scenariodoc

import (
	"reflect"
	"testing"

	"github.com/zyra/gobac/v2/simulator"
)

func TestAddDeviceAssignsNextFreeID(t *testing.T) {
	doc := New() // already has device id 1
	if doc.Dirty() {
		t.Fatal("New() document should not start dirty")
	}

	dev := doc.AddDevice()
	if dev.ID != 2 || dev.Name != "device-2" {
		t.Fatalf("AddDevice() = %+v, want ID:2 Name:device-2", dev)
	}
	if !doc.Dirty() {
		t.Fatal("AddDevice should mark the document dirty")
	}
	if len(doc.Scenario().Devices) != 2 {
		t.Fatalf("len(Devices) = %d, want 2", len(doc.Scenario().Devices))
	}
}

func TestAddDeviceFillsGapBeforeAppending(t *testing.T) {
	doc := New()
	doc.RemoveDevice(0) // remove id 1, leaving zero devices
	if len(doc.Scenario().Devices) != 0 {
		t.Fatalf("RemoveDevice did not remove the device: %+v", doc.Scenario().Devices)
	}

	dev := doc.AddDevice()
	if dev.ID != 1 || dev.Name != "device-1" {
		t.Fatalf("AddDevice() after clearing = %+v, want ID:1 Name:device-1", dev)
	}
}

func TestRemoveDeviceOutOfRangeIsNoOp(t *testing.T) {
	doc := New()
	before := len(doc.Scenario().Devices)
	doc.RemoveDevice(5)
	if len(doc.Scenario().Devices) != before {
		t.Fatalf("out-of-range RemoveDevice changed device count: %d -> %d", before, len(doc.Scenario().Devices))
	}
	if doc.Dirty() {
		t.Fatal("out-of-range RemoveDevice should not mark the document dirty")
	}
}

func TestAddObjectMultiStateSeedsValidState(t *testing.T) {
	doc := New()

	obj := doc.AddObject(0, "multi-state-value")
	if obj == nil {
		t.Fatal("AddObject returned nil")
	}

	want := simulator.ObjectSpec{
		Type:           "multi-state-value",
		Instance:       1,
		Name:           "multi-state-value-1",
		PresentValue:   uint32(1),
		NumberOfStates: 2,
	}
	if !reflect.DeepEqual(*obj, want) {
		t.Fatalf("AddObject(0, \"multi-state-value\") = %+v, want %+v", *obj, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate() after adding a fresh multi-state-value: %v", err)
	}
}

func TestAddObjectNonMultiStateHasNoStateFields(t *testing.T) {
	doc := New()
	obj := doc.AddObject(0, "analog-value")
	want := simulator.ObjectSpec{
		Type:     "analog-value",
		Instance: 1,
		Name:     "analog-value-1",
	}
	if !reflect.DeepEqual(*obj, want) {
		t.Fatalf("AddObject(0, \"analog-value\") = %+v, want %+v", *obj, want)
	}
}

func TestAddObjectAcceptsNonHyphenatedMultiStateAlias(t *testing.T) {
	doc := New()
	obj := doc.AddObject(0, "multistate-input")
	if obj == nil || obj.Type != "multi-state-input" {
		t.Fatalf("AddObject(0, \"multistate-input\") = %+v, want canonical type multi-state-input", obj)
	}
}

func TestAddObjectNextFreeInstancePerType(t *testing.T) {
	doc := New()
	doc.AddObject(0, "analog-value") // instance 1
	doc.RemoveObject(0, 0)
	second := doc.AddObject(0, "analog-value")
	if second.Instance != 1 {
		t.Fatalf("instance after remove+add = %d, want 1 (smallest free)", second.Instance)
	}

	third := doc.AddObject(0, "analog-value")
	if third.Instance != 2 {
		t.Fatalf("instance of second analog-value = %d, want 2", third.Instance)
	}

	// A different type on the same device starts its own instance
	// numbering at 1.
	binary := doc.AddObject(0, "binary-value")
	if binary.Instance != 1 {
		t.Fatalf("instance of first binary-value = %d, want 1", binary.Instance)
	}
}

func TestAddObjectInvalidTypeOrDeviceIndexReturnsNil(t *testing.T) {
	doc := New()
	if obj := doc.AddObject(0, "not-a-real-type"); obj != nil {
		t.Fatalf("AddObject with an invalid type = %+v, want nil", obj)
	}
	if obj := doc.AddObject(9, "analog-value"); obj != nil {
		t.Fatalf("AddObject with an out-of-range device index = %+v, want nil", obj)
	}
}

func TestRemoveObjectOutOfRangeIsNoOp(t *testing.T) {
	doc := New()
	doc.AddObject(0, "analog-value")
	before := len(doc.Scenario().Devices[0].Objects)

	doc.RemoveObject(0, 5)
	doc.RemoveObject(5, 0)

	if len(doc.Scenario().Devices[0].Objects) != before {
		t.Fatalf("out-of-range RemoveObject changed object count: %d -> %d", before, len(doc.Scenario().Devices[0].Objects))
	}
}
