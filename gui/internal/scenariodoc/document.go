// Package scenariodoc is a UI-free editing model over simulator.Scenario.
// It never defines a parallel schema: loading, editing, and saving all go
// through the simulator package's own decode/validate logic, so a Document
// can never write a file the simulator itself would reject (see
// gui-architecture.md §4.4 and task G6). It has no dependency on Fyne and
// must remain unit-testable on its own (see the global constraints in
// gui-architecture.md §6.3).
package scenariodoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyra/gobac/v2/simulator"
	"gopkg.in/yaml.v2"
)

// ErrNoPath is returned by Save when the document has never been saved to a
// destination path (use SaveAs to give it a first one).
var ErrNoPath = errors.New("document has no destination path; use SaveAs")

// Document holds one scenario being edited: its live simulator.Scenario
// value, the path/format it was last loaded from or saved to (if any), and
// whether it has unsaved edits.
type Document struct {
	path     string
	format   string // "json", or "yaml"/"yml"/"" (all treated as YAML)
	scenario simulator.Scenario
	dirty    bool
}

// New returns a minimal, already-valid Document: scenario version 1,
// single-device network mode, and one device {id: 1, name: "device-1"}. It
// has no destination path until SaveAs is called.
func New() *Document {
	return &Document{
		format: "yaml",
		scenario: simulator.Scenario{
			Version: simulator.ScenarioVersion,
			Network: simulator.NetworkConfig{Mode: "single-device"},
			Devices: []simulator.DeviceSpec{{ID: 1, Name: "device-1"}},
		},
	}
}

// Load reads and decodes the scenario at path. The format is chosen from
// the file extension: ".json" decodes as JSON, anything else (including no
// extension) as YAML — the same rule cmd/gobac-sim uses. The returned
// Document is not dirty.
func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	format := formatForPath(path)
	scenario, err := simulator.DecodeScenario(bytes.NewReader(data), format)
	if err != nil {
		return nil, err
	}
	return &Document{path: path, format: format, scenario: *scenario}, nil
}

// formatForPath derives a DecodeScenario/marshal format from a file
// extension: the trimmed, lowercased extension (e.g. "yaml", "yml", "json",
// or "" for an extensionless path) — mirroring cmd/gobac-sim/main.go.
func formatForPath(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

// Scenario returns the live, mutable scenario this Document holds. Callers
// that mutate it directly (as opposed to through the helpers in mutate.go)
// are responsible for calling MarkDirty themselves.
func (d *Document) Scenario() *simulator.Scenario {
	return &d.scenario
}

// Path returns the document's current destination path, or "" if it has
// never been saved (a fresh Document from New, or one that failed every
// SaveAs so far).
func (d *Document) Path() string {
	return d.path
}

// Dirty reports whether the scenario has edits that have not been saved.
func (d *Document) Dirty() bool {
	return d.dirty
}

// MarkDirty flags the document as having unsaved edits.
func (d *Document) MarkDirty() {
	d.dirty = true
}

// Validate reports whether the current scenario is valid, after applying
// the same normalization (default-filling) the simulator itself applies on
// load. It does this by marshaling a copy of the scenario and decoding it
// back through simulator.DecodeScenario, so it exercises exactly the
// simulator's own Scenario.Validate — never a parallel rule set — without
// mutating the live scenario held by Document.
func (d *Document) Validate() error {
	data, err := yaml.Marshal(&d.scenario)
	if err != nil {
		return err
	}
	_, err = simulator.DecodeScenario(bytes.NewReader(data), "yaml")
	return err
}

// Save writes the scenario back to its current path (set by Load or a
// previous successful SaveAs) in its current format. It returns ErrNoPath
// if the document has no destination yet.
func (d *Document) Save() error {
	if d.path == "" {
		return ErrNoPath
	}
	return d.writeTo(d.path, d.format)
}

// SaveAs writes the scenario to path (format chosen from its extension, the
// same rule as Load) and, only on success, makes path the document's new
// destination and format.
func (d *Document) SaveAs(path string) error {
	format := formatForPath(path)
	if err := d.writeTo(path, format); err != nil {
		return err
	}
	d.path = path
	d.format = format
	return nil
}

// writeTo validates the scenario, marshals it, then re-decodes and
// re-validates the exact bytes it is about to write — guaranteeing Save
// (structurally) never produces a file the simulator would reject — and
// finally writes them atomically (temp file + rename) so a failure at any
// step never touches (or corrupts) an existing file at path.
func (d *Document) writeTo(path, format string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	data, err := marshalScenario(&d.scenario, format)
	if err != nil {
		return err
	}
	if _, err := simulator.DecodeScenario(bytes.NewReader(data), format); err != nil {
		return fmt.Errorf("internal error: scenario failed round-trip validation before save: %w", err)
	}
	if err := atomicWrite(path, data); err != nil {
		return err
	}
	d.dirty = false
	return nil
}

// marshalScenario serializes s per format, mirroring the "json" vs.
// everything-else-is-YAML split simulator.DecodeScenario uses.
func marshalScenario(s *simulator.Scenario, format string) ([]byte, error) {
	if strings.ToLower(format) == "json" {
		return json.MarshalIndent(s, "", "  ")
	}
	return yaml.Marshal(s)
}

// atomicWrite writes data to path via a temp file created in the same
// directory followed by a rename, so a failure (an unwritable directory, a
// full disk, ...) never leaves a partially-written file at path and never
// disturbs whatever was already there.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".scenariodoc-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
