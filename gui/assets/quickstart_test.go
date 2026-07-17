package assets

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/simulator"
)

// TestQuickstartScenarioDecodesAndValidates guards against asset drift: the
// bundled quickstart.yaml must always be a scenario the simulator itself
// accepts, since internal/simrun and the Quickstart view both decode it
// as-is with no GUI-side patching.
func TestQuickstartScenarioDecodesAndValidates(t *testing.T) {
	sc, err := simulator.DecodeScenario(bytes.NewReader(QuickstartScenario), "yaml")
	if err != nil {
		t.Fatalf("DecodeScenario: %v", err)
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if got, want := sc.Network.Mode, "multi-port"; got != want {
		t.Errorf("network mode = %q, want %q", got, want)
	}
	if len(sc.Devices) != 3 {
		t.Fatalf("len(Devices) = %d, want 3", len(sc.Devices))
	}

	wantIDs := map[uint32]bool{1001: true, 1002: true, 1003: true}
	seenPorts := make(map[uint16]int)
	for _, d := range sc.Devices {
		if !wantIDs[d.ID] {
			t.Errorf("unexpected device id %d", d.ID)
		}
		if d.Port == 0 {
			t.Errorf("device %d has an unset port", d.ID)
		}
		seenPorts[d.Port]++
	}
	for port, count := range seenPorts {
		if count != 1 {
			t.Errorf("port %d used by %d devices, want distinct ports per device", port, count)
		}
	}
}
