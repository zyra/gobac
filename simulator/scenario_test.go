package simulator

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestDecodeAndBuildYAMLScenario(t *testing.T) {
	input := `
version: 1
network:
  mode: multi-ip
  port: 47808
devices:
  - id: 1001
    address: 127.0.0.2
    name: AHU-1
    vendor_id: 999
    objects:
      - type: analog-input
        instance: 1
        name: Supply Air Temperature
        present_value: 21.5
        cov_increment: 0.25
      - type: binary-output
        instance: 1
        name: Supply Fan Command
        present_value: inactive
        writable: true
        commandable: true
`
	scenario, err := DecodeScenario(strings.NewReader(input), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		t.Fatal(err)
	}
	device, err := network.Device(1001)
	if err != nil {
		t.Fatal(err)
	}
	values, err := device.ReadProperty(
		ObjectID{Type: uint16(types.ObjectTypeAnalogInput), Instance: 1},
		types.PropertyPresentValue,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := values[0].Value; got != float32(21.5) {
		t.Fatalf("present value = %v, want 21.5", got)
	}
}

func TestNumericUint32RejectsOverflow(t *testing.T) {
	overflow := int64(math.MaxUint32) + 1
	values := []interface{}{uint64(overflow), overflow, float64(overflow)}
	if strconv.IntSize == 64 {
		values = append(values, int(overflow))
	}
	for _, value := range values {
		if _, err := numericUint32(value); err == nil {
			t.Fatalf("overflowing value %T(%v) was accepted", value, value)
		}
	}
}

func TestDecodeScenarioRejectsOverflowingPresentValue(t *testing.T) {
	input := `
version: 1
network:
  mode: single-device
devices:
  - id: 1
    name: device
    objects:
      - type: multi-state-value
        instance: 1
        name: state
        number_of_states: 3
        present_value: 4294967297
`
	_, err := DecodeScenario(strings.NewReader(input), "yaml")
	if err == nil {
		t.Fatal("overflowing present value was accepted")
	}
	if !strings.Contains(err.Error(), "exceeds the unsigned 32-bit range") {
		t.Fatalf("expected uint32 overflow error, got: %v", err)
	}
}

func TestScenarioRejectsMultiStateValuesOutsideDeclaredRange(t *testing.T) {
	scenario := &Scenario{
		Version: ScenarioVersion,
		Network: NetworkConfig{Mode: "single-device", Port: 47808},
		Devices: []DeviceSpec{{
			ID: 1, Name: "device", Objects: []ObjectSpec{{
				Type: "multi-state-output", Instance: 1, Name: "state",
				PresentValue: uint64(4), NumberOfStates: 3,
			}},
		}},
	}
	if err := scenario.Validate(); err == nil {
		t.Fatal("present value above number_of_states was accepted")
	}
	scenario.Devices[0].Objects[0].PresentValue = uint64(1)
	scenario.Devices[0].Objects[0].NumberOfStates = 0
	if err := scenario.Validate(); err == nil {
		t.Fatal("multi-state object without number_of_states was accepted")
	}
}

func TestScenarioRejectsNegativeCOVIncrement(t *testing.T) {
	scenario := &Scenario{
		Version: ScenarioVersion,
		Network: NetworkConfig{Mode: "single-device", Port: 47808},
		Devices: []DeviceSpec{{
			ID: 1, Name: "device", Objects: []ObjectSpec{{
				Type: "analog-value", Instance: 1, Name: "value", COVIncrement: -1,
			}},
		}},
	}
	if err := scenario.Validate(); err == nil {
		t.Fatal("negative cov_increment was accepted")
	}
}

func TestCommandableScenarioStartsAtConfiguredPresentValue(t *testing.T) {
	scenario := &Scenario{
		Version: ScenarioVersion,
		Network: NetworkConfig{Mode: "single-device", Port: 47808},
		Devices: []DeviceSpec{{
			ID: 1, Name: "device", Objects: []ObjectSpec{{
				Type: "analog-output", Instance: 1, Name: "command",
				PresentValue: float64(35), Writable: true, Commandable: true,
				RelinquishDefault: float64(0),
			}},
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
	object := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 1}
	values, err := device.ReadProperty(object, uint32(types.PropertyPresentValue), nil)
	if err != nil || values[0].Value != float32(35) {
		t.Fatalf("initial present value = %+v, %v", values, err)
	}
	if err := device.WriteProperty(object, uint32(types.PropertyPresentValue), []Value{{Tag: types.TagNull}}, 16); err != nil {
		t.Fatal(err)
	}
	values, err = device.ReadProperty(object, uint32(types.PropertyPresentValue), nil)
	if err != nil || values[0].Value != float32(0) {
		t.Fatalf("relinquished present value = %+v, %v", values, err)
	}
}

func TestDecodeScenarioRejectsUnknownFields(t *testing.T) {
	_, err := DecodeScenario(strings.NewReader("version: 1\nunknown: true\ndevices: []\n"), "yaml")
	if err == nil {
		t.Fatal("expected strict YAML decoding error")
	}
}

func TestDecodeScenarioRejectsSeedField(t *testing.T) {
	input := "version: 1\nseed: 42\nnetwork:\n  mode: single-device\ndevices:\n  - id: 1\n    name: device\n"
	_, err := DecodeScenario(strings.NewReader(input), "yaml")
	if err == nil {
		t.Fatal("scenario with a seed field was accepted")
	}
	if !strings.Contains(err.Error(), "seed") {
		t.Fatalf("expected unknown-field error mentioning seed, got: %v", err)
	}
}

func TestDecodeScenarioRejectsTrailingJSON(t *testing.T) {
	input := `{"version":1,"network":{"mode":"single-device"},"devices":[{"id":1,"name":"one"}]} {}`
	if _, err := DecodeScenario(strings.NewReader(input), "json"); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestScenarioRejectsDuplicateEndpoints(t *testing.T) {
	scenario := &Scenario{
		Version: ScenarioVersion,
		Network: NetworkConfig{Mode: "multi-ip", Port: 47808},
		Devices: []DeviceSpec{
			{ID: 1, Name: "one", Address: "127.0.0.2", Port: 47808},
			{ID: 2, Name: "two", Address: "127.0.0.2", Port: 47808},
		},
	}
	if err := scenario.Validate(); err == nil {
		t.Fatal("expected duplicate endpoint validation error")
	}
}

func TestApplyScenarioDefaults(t *testing.T) {
	input := `
devices:
  - id: 1
    name: device
`
	scenario, err := DecodeScenario(strings.NewReader(input), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Version != 1 {
		t.Fatalf("version = %d, want 1", scenario.Version)
	}
	if scenario.Network.Mode != "single-device" {
		t.Fatalf("network mode = %q, want single-device", scenario.Network.Mode)
	}
	if scenario.Network.Port != 0xbac0 {
		t.Fatalf("network port = %#x, want 0xbac0", scenario.Network.Port)
	}
	if scenario.Devices[0].Port != 0xbac0 {
		t.Fatalf("device port = %#x, want 0xbac0", scenario.Devices[0].Port)
	}
	if scenario.Devices[0].VendorName != "GoBAC" {
		t.Fatalf("vendor name = %q, want GoBAC", scenario.Devices[0].VendorName)
	}
	if scenario.Devices[0].ModelName != "GoBAC Simulator" {
		t.Fatalf("model name = %q, want GoBAC Simulator", scenario.Devices[0].ModelName)
	}
	if scenario.Devices[0].ProtocolRevision != 14 {
		t.Fatalf("protocol revision = %d, want 14", scenario.Devices[0].ProtocolRevision)
	}

	multiInput := `
devices:
  - id: 1
    name: one
    address: 127.0.0.2
  - id: 2
    name: two
    address: 127.0.0.3
`
	multiScenario, err := DecodeScenario(strings.NewReader(multiInput), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if multiScenario.Network.Mode != "multi-ip" {
		t.Fatalf("multi-device network mode = %q, want multi-ip", multiScenario.Network.Mode)
	}
}

func TestObjectTypeNumberRejectsUnsupportedType(t *testing.T) {
	_, err := objectTypeNumber("thermostat")
	if err == nil || !strings.Contains(err.Error(), `"thermostat" is not supported`) {
		t.Fatalf("unsupported object type error = %v", err)
	}

	// Same function, locking in the trim/underscore/lowercase normalization.
	number, err := objectTypeNumber(" Multi_State_Value ")
	if err != nil {
		t.Fatalf("normalized object type: %v", err)
	}
	if number != uint16(types.ObjectTypeMultiStateValue) {
		t.Fatalf("normalized object type = %d, want %d", number, uint16(types.ObjectTypeMultiStateValue))
	}
}

func TestNormalizePresentValueRejectsNonNumericAnalog(t *testing.T) {
	_, _, err := normalizePresentValue(uint16(types.ObjectTypeAnalogInput), "warm")
	if err == nil || !strings.Contains(err.Error(), "is not numeric") {
		t.Fatalf("non-numeric analog present value error = %v", err)
	}

	// End-to-end: prove Validate calls normalizePresentValue via DecodeScenario.
	input := `
devices:
  - id: 1
    name: device
    objects:
      - type: analog-input
        instance: 1
        name: sensor
        present_value: warm
`
	if _, err := DecodeScenario(strings.NewReader(input), "yaml"); err == nil || !strings.Contains(err.Error(), "is not numeric") {
		t.Fatalf("end-to-end non-numeric analog present value error = %v", err)
	}
}
