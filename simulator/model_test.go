package simulator

import (
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
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
	if err := property.Write([]Value{{Tag: 4, Value: float32(21)}}, 6); err != ErrReservedPriority {
		t.Fatalf("priority 6 error = %v", err)
	}
}

func TestMultiStatePropertyEnforcesNumberOfStates(t *testing.T) {
	property := &Property{
		ID:              85,
		Writable:        true,
		Values:          []Value{{Tag: 2, Value: uint32(1)}},
		MinimumUnsigned: 1,
		MaximumUnsigned: 3,
		Scalar:          true,
		ExpectedTag:     types.TagUnsigned,
	}
	if err := property.Write([]Value{{Tag: 2, Value: uint32(4)}}, 0); err != ErrValueOutOfRange {
		t.Fatalf("out-of-range multi-state write error = %v", err)
	}
	if err := property.Write([]Value{{Tag: 2, Value: uint32(3)}}, 0); err != nil {
		t.Fatalf("valid multi-state write: %v", err)
	}
}

func TestScalarPropertiesRejectWrongBACnetType(t *testing.T) {
	tests := []struct {
		name     string
		expected uint8
		wrong    Value
	}{
		{"analog", types.TagReal, Value{Tag: types.TagCharacterString, Value: "wrong"}},
		{"binary", types.TagEnumerated, Value{Tag: types.TagBoolean, Value: true}},
		{"multi-state", types.TagUnsigned, Value{Tag: types.TagEnumerated, Value: uint32(1)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			property := &Property{Writable: true, Scalar: true, ExpectedTag: test.expected}
			if err := property.Write([]Value{test.wrong}, 0); err != ErrInvalidDataType {
				t.Fatalf("wrong type error = %v", err)
			}
			if err := property.Write([]Value{{Tag: test.expected, Value: uint32(1)}, {Tag: test.expected, Value: uint32(2)}}, 0); err != ErrInvalidDataType {
				t.Fatalf("multiple scalar values error = %v", err)
			}
		})
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
