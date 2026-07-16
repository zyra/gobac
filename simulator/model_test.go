package simulator

import (
	"strings"
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

func TestCommandablePriorityArrayReads(t *testing.T) {
	scenario := &Scenario{
		Version: ScenarioVersion,
		Network: NetworkConfig{Mode: "single-device", Port: 47808},
		Devices: []DeviceSpec{{
			ID: 1, Name: "device", Objects: []ObjectSpec{
				{
					Type: "analog-output", Instance: 1, Name: "command",
					PresentValue: float64(35), Writable: true, Commandable: true,
					RelinquishDefault: float64(0),
				},
				{
					Type: "analog-input", Instance: 2, Name: "sensor",
					PresentValue: float64(10),
				},
			},
		}},
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		t.Fatal(err)
	}
	device, err := network.Device(1)
	if err != nil {
		t.Fatal(err)
	}
	commandable := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 1}
	nonCommandable := ObjectID{Type: uint16(types.ObjectTypeAnalogInput), Instance: 2}

	values, err := device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), nil)
	if err != nil {
		t.Fatalf("read priority array: %v", err)
	}
	if len(values) != PrioritySlots {
		t.Fatalf("priority array length = %d, want %d", len(values), PrioritySlots)
	}
	for i := 0; i < PrioritySlots-1; i++ {
		if values[i].Tag != types.TagNull || values[i].Value != nil {
			t.Fatalf("slot %d = %+v, want NULL", i+1, values[i])
		}
	}
	if values[PrioritySlots-1].Tag != types.TagReal || values[PrioritySlots-1].Value != float32(35) {
		t.Fatalf("slot 16 = %+v, want relinquish-priority value 35", values[PrioritySlots-1])
	}

	if err := device.WriteProperty(commandable, uint32(types.PropertyPresentValue), []Value{{Tag: types.TagReal, Value: float32(21)}}, 8); err != nil {
		t.Fatal(err)
	}

	index := uint32(8)
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Tag != types.TagReal || values[0].Value != float32(21) {
		t.Fatalf("slot 8 (index 8) = %+v, want 21", values)
	}

	index = 16
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Value != float32(35) {
		t.Fatalf("slot 16 (index 16) = %+v, want 35", values)
	}

	index = 3
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Tag != types.TagNull {
		t.Fatalf("slot 3 (index 3) = %+v, want NULL", values)
	}

	index = 0
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Tag != 2 || values[0].Value != uint32(16) {
		t.Fatalf("array length (index 0) = %+v, want 16", values)
	}

	index = 17
	if _, err := device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index); err != ErrInvalidArrayIndex {
		t.Fatalf("index 17 error = %v, want ErrInvalidArrayIndex", err)
	}

	// Relinquish priority 8 and confirm the array reflects live state, not a
	// stale snapshot.
	if err := device.WriteProperty(commandable, uint32(types.PropertyPresentValue), []Value{{Tag: types.TagNull}}, 8); err != nil {
		t.Fatal(err)
	}
	index = 8
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), &index)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Tag != types.TagNull {
		t.Fatalf("slot 8 after relinquish = %+v, want NULL", values)
	}

	// Mutation-safety: mutating a returned value must not affect stored state.
	values, err = device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), nil)
	if err != nil {
		t.Fatal(err)
	}
	values[PrioritySlots-1].Value = float32(999)
	again, err := device.ReadProperty(commandable, uint32(types.PropertyPriorityArray), nil)
	if err != nil {
		t.Fatal(err)
	}
	if again[PrioritySlots-1].Value != float32(35) {
		t.Fatalf("mutation leaked into stored state: %+v", again[PrioritySlots-1])
	}

	// Negative: a non-commandable object has no Priority_Array.
	if _, err := device.ReadProperty(nonCommandable, uint32(types.PropertyPriorityArray), nil); err != ErrUnknownProperty {
		t.Fatalf("non-commandable priority array error = %v, want ErrUnknownProperty", err)
	}

	// Write denial: Priority_Array is read-only.
	if err := device.WriteProperty(commandable, uint32(types.PropertyPriorityArray), []Value{{Tag: types.TagReal, Value: float32(1)}}, 0); err != ErrWriteDenied {
		t.Fatalf("write to priority array error = %v, want ErrWriteDenied", err)
	}
}

func TestReadNonArrayPropertyWithIndexFails(t *testing.T) {
	property := &Property{ID: 85, Values: []Value{{Tag: 4, Value: float32(1)}}}

	index := uint32(1)
	if _, err := property.Read(&index); err != ErrPropertyNotArray {
		t.Fatalf("non-array read with index error = %v, want ErrPropertyNotArray", err)
	}

	// Guard against over-eager rejection: a nil index must still read fine.
	if _, err := property.Read(nil); err != nil {
		t.Fatalf("non-array read with nil index: %v", err)
	}
}

func TestReadArrayPropertyOutOfRangeIndexFails(t *testing.T) {
	property := &Property{
		ID:    76,
		Array: true,
		Values: []Value{
			{Tag: 12, Value: uint32(1)},
			{Tag: 12, Value: uint32(2)},
		},
	}

	index := uint32(3)
	if _, err := property.Read(&index); err != ErrInvalidArrayIndex {
		t.Fatalf("out-of-range array index error = %v, want ErrInvalidArrayIndex", err)
	}

	// BacnetArrayAll (all-ones index) is the whole-array escape hatch on the
	// same branch chain and must not be treated as out of range.
	index = ^uint32(0)
	values, err := property.Read(&index)
	if err != nil {
		t.Fatalf("BacnetArrayAll read: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("BacnetArrayAll length = %d, want 2", len(values))
	}
}

func TestNetworkAddDeviceGuards(t *testing.T) {
	network := NewNetwork()

	if err := network.AddDevice(nil); err == nil || err.Error() != "device is required" {
		t.Fatalf("nil device error = %v, want %q", err, "device is required")
	}

	if err := network.AddDevice(&Device{ID: MaxObjectInstance + 1}); err == nil || !strings.Contains(err.Error(), "exceeds BACnet object-instance range") {
		t.Fatalf("out-of-range device id error = %v", err)
	}

	if err := network.AddDevice(&Device{ID: 7}); err != nil {
		t.Fatalf("add device 7: %v", err)
	}
	if err := network.AddDevice(&Device{ID: 7}); err == nil || !strings.Contains(err.Error(), "duplicate device id 7") {
		t.Fatalf("duplicate device id error = %v", err)
	}
	if _, err := network.Device(7); err != nil {
		t.Fatalf("device 7 should still be retrievable: %v", err)
	}
}
