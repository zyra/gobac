package simulator

import (
	"testing"
)

func TestCommandablePropertyPriorityResolution(t *testing.T) {
	defaultValue := Value{Tag: 4, Value: float32(18)}
	property := &Property{ID: 85, Writable: true, RelinquishDefault: &defaultValue}

	if err := property.Write([]Value{{Tag: 4, Value: float32(21)}}, 8); err != nil {
		t.Fatal(err)
	}
	if err := property.Write([]Value{{Tag: 4, Value: float32(23)}}, 4); err != nil {
		t.Fatal(err)
	}

	values, err := property.Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := values[0].Value; got != float32(23) {
		t.Fatalf("priority 4 should win: got %v", got)
	}

	if err := property.Write([]Value{{Tag: 0}}, 4); err != nil {
		t.Fatal(err)
	}
	values, err = property.Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := values[0].Value; got != float32(21) {
		t.Fatalf("priority 8 should be restored after relinquish: got %v", got)
	}

	if err := property.Write([]Value{{Tag: 0}}, 8); err != nil {
		t.Fatal(err)
	}
	values, err = property.Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := values[0].Value; got != float32(18) {
		t.Fatalf("relinquish default should be restored: got %v", got)
	}
}

func TestCommandablePropertyRejectsReservedPrioritySix(t *testing.T) {
	defaultValue := Value{Tag: 4, Value: float32(18)}
	property := &Property{ID: 85, Writable: true, RelinquishDefault: &defaultValue}
	if err := property.Write([]Value{{Tag: 4, Value: float32(21)}}, 6); err != ErrInvalidPriority {
		t.Fatalf("priority 6 error = %v", err)
	}
}

func TestArrayPropertyReads(t *testing.T) {
	property := &Property{
		ID:    76,
		Array: true,
		Values: []Value{
			{Tag: 12, Value: uint32(1)},
			{Tag: 12, Value: uint32(2)},
		},
	}

	index := uint32(0)
	values, err := property.Read(&index)
	if err != nil {
		t.Fatal(err)
	}
	if values[0].Value != uint32(2) {
		t.Fatalf("array length = %v, want 2", values[0].Value)
	}

	index = 2
	values, err = property.Read(&index)
	if err != nil {
		t.Fatal(err)
	}
	if values[0].Value != uint32(2) {
		t.Fatalf("array element = %v, want 2", values[0].Value)
	}
}

func TestDevicePropertyErrors(t *testing.T) {
	device := &Device{Objects: map[ObjectID]*Object{}}
	_, err := device.ReadProperty(ObjectID{Type: 2, Instance: 1}, 85, nil)
	if err != ErrUnknownObject {
		t.Fatalf("got %v, want ErrUnknownObject", err)
	}
}
