package simulator

import (
	"strings"
	"testing"

	"github.com/zyra/gobac/bacnet/types"
)

func TestDecodeAndBuildYAMLScenario(t *testing.T) {
	input := `
version: 1
seed: 42
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

func TestDecodeScenarioRejectsUnknownFields(t *testing.T) {
	_, err := DecodeScenario(strings.NewReader("version: 1\nunknown: true\ndevices: []\n"), "yaml")
	if err == nil {
		t.Fatal("expected strict YAML decoding error")
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
