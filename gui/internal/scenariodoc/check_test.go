package scenariodoc

import (
	"strings"
	"testing"

	"github.com/zyra/gobac/v2/simulator"
)

func TestFieldErrorsInitialPriorityReserved(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{
			ID:   1,
			Name: "dev-1",
			Objects: []simulator.ObjectSpec{{
				Type:            "analog-value",
				Instance:        1,
				Name:            "obj-1",
				Commandable:     true,
				Writable:        true,
				PresentValue:    21.5,
				InitialPriority: 6,
			}},
		}},
	}

	errs := FieldErrors(s)
	msg, ok := errs["devices[0].objects[0].initial_priority"]
	if !ok {
		t.Fatalf("FieldErrors missing key devices[0].objects[0].initial_priority; got %v", errs)
	}
	if !strings.Contains(msg, "6") {
		t.Fatalf("initial_priority message %q does not mention 6", msg)
	}
}

func TestFieldErrorsDuplicateObjectInstance(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{
			ID:   1,
			Name: "dev-1",
			Objects: []simulator.ObjectSpec{
				{Type: "analog-value", Instance: 1, Name: "first"},
				{Type: "analog-value", Instance: 1, Name: "second"},
			},
		}},
	}

	errs := FieldErrors(s)
	if _, ok := errs["devices[0].objects[1].instance"]; !ok {
		t.Fatalf("FieldErrors missing key devices[0].objects[1].instance; got %v", errs)
	}
	if _, ok := errs["devices[0].objects[0].instance"]; ok {
		t.Fatalf("FieldErrors flagged the first (non-duplicate) occurrence too: %v", errs)
	}
}

func TestFieldErrorsMultiIPRequiresAddress(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "multi-ip"},
		Devices: []simulator.DeviceSpec{{
			ID:   1,
			Name: "dev-1",
		}},
	}

	errs := FieldErrors(s)
	if _, ok := errs["devices[0].address"]; !ok {
		t.Fatalf("FieldErrors missing key devices[0].address; got %v", errs)
	}
}

func TestFieldErrorsValidScenarioIsEmpty(t *testing.T) {
	doc := New()
	errs := FieldErrors(doc.Scenario())
	if len(errs) != 0 {
		t.Fatalf("FieldErrors on a fresh valid scenario = %v, want empty", errs)
	}
}

func TestFieldErrorsDuplicateDeviceID(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "multi-port"},
		Devices: []simulator.DeviceSpec{
			{ID: 1, Name: "a"},
			{ID: 1, Name: "b"},
		},
	}
	errs := FieldErrors(s)
	if _, ok := errs["devices[1].id"]; !ok {
		t.Fatalf("FieldErrors missing key devices[1].id for duplicate device id; got %v", errs)
	}
}

func TestFieldErrorsDeviceIDExceedsRange(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{ID: simulator.MaxObjectInstance + 1, Name: "dev"}},
	}
	errs := FieldErrors(s)
	if _, ok := errs["devices[0].id"]; !ok {
		t.Fatalf("FieldErrors missing key devices[0].id for out-of-range id; got %v", errs)
	}
}

func TestFieldErrorsSingleDeviceModeRequiresExactlyOneDevice(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}},
	}
	errs := FieldErrors(s)
	if _, ok := errs["devices"]; !ok {
		t.Fatalf("FieldErrors missing key \"devices\" for single-device mode with 2 devices; got %v", errs)
	}
}

func TestFieldErrorsMultiStateNumberOfStatesRequired(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{
			ID:   1,
			Name: "dev-1",
			Objects: []simulator.ObjectSpec{{
				Type:     "multi-state-value",
				Instance: 1,
				Name:     "obj-1",
			}},
		}},
	}
	errs := FieldErrors(s)
	if _, ok := errs["devices[0].objects[0].number_of_states"]; !ok {
		t.Fatalf("FieldErrors missing key devices[0].objects[0].number_of_states; got %v", errs)
	}
}

func TestFieldErrorsMultiStatePresentValueExceedsStates(t *testing.T) {
	s := &simulator.Scenario{
		Version: 1,
		Network: simulator.NetworkConfig{Mode: "single-device"},
		Devices: []simulator.DeviceSpec{{
			ID:   1,
			Name: "dev-1",
			Objects: []simulator.ObjectSpec{{
				Type:           "multi-state-value",
				Instance:       1,
				Name:           "obj-1",
				NumberOfStates: 2,
				PresentValue:   uint32(5),
			}},
		}},
	}
	errs := FieldErrors(s)
	if _, ok := errs["devices[0].objects[0].present_value"]; !ok {
		t.Fatalf("FieldErrors missing key devices[0].objects[0].present_value; got %v", errs)
	}
}
