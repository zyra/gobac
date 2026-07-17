package scenariodoc

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/zyra/gobac/v2/simulator"
)

func TestLoadSaveReloadRoundTrips(t *testing.T) {
	doc, err := Load("testdata/roundtrip.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out.yaml")
	if err := doc.SaveAs(dst); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	reloaded, err := Load(dst)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}

	if !reflect.DeepEqual(doc.Scenario(), reloaded.Scenario()) {
		t.Fatalf("round-trip mismatch:\noriginal: %#v\nreloaded: %#v", doc.Scenario(), reloaded.Scenario())
	}
}

func TestNewDocumentValidatesAndSaves(t *testing.T) {
	doc := New()
	if err := doc.Validate(); err != nil {
		t.Fatalf("New().Validate(): %v", err)
	}

	dst := filepath.Join(t.TempDir(), "fresh.yaml")
	if err := doc.SaveAs(dst); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, err := simulator.DecodeScenario(strings.NewReader(string(data)), "yaml"); err != nil {
		t.Fatalf("saved file fails simulator decode+validate: %v", err)
	}
}

func TestSaveRoundTripsThroughSimulatorDecode(t *testing.T) {
	doc, err := Load("testdata/roundtrip.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "out.yaml")
	if err := doc.SaveAs(dst); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, err := simulator.DecodeScenario(strings.NewReader(string(data)), "yaml"); err != nil {
		t.Fatalf("saved file fails simulator decode+validate: %v", err)
	}
}

func TestSaveIsAtomicAndLeavesDestinationUntouchedOnFailure(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "existing.yaml")
	const sentinel = "sentinel content, do not overwrite\n"
	if err := os.WriteFile(dst, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	doc := New()
	// Force Validate() to fail without touching the destination file.
	doc.Scenario().Version = 2

	err := doc.SaveAs(dst)
	if err == nil {
		t.Fatal("SaveAs succeeded despite an invalid scenario")
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile after failed SaveAs: %v", err)
	}
	if string(data) != sentinel {
		t.Fatalf("destination file was modified: got %q, want sentinel intact", data)
	}
}

func TestSaveToUnwritableDirectoryFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows: Unix permission bits on a directory do not block file creation")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not block writes")
	}
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	doc := New()
	dst := filepath.Join(roDir, "scenario.yaml")
	if err := doc.SaveAs(dst); err == nil {
		t.Fatal("SaveAs into an unwritable directory succeeded")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination file unexpectedly exists: err=%v", err)
	}
}

func TestSaveWithoutDestinationReturnsErrNoPath(t *testing.T) {
	doc := New()
	if err := doc.Save(); err != ErrNoPath {
		t.Fatalf("Save() on a document with no path = %v, want ErrNoPath", err)
	}
}

func TestSaveAsJSONRoundTrips(t *testing.T) {
	// Note: interface{}-typed fields (present_value, relinquish_default)
	// decode as `int` from YAML (gopkg.in/yaml.v2) but as `float64` from
	// JSON (encoding/json) — a property of the underlying decoders, not of
	// Document. So this asserts the JSON path is lossless on its own terms
	// (JSON -> Document -> JSON -> Document is DeepEqual), rather than
	// comparing against a scenario that was originally decoded from YAML.
	doc, err := Load("testdata/roundtrip.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	firstPath := filepath.Join(t.TempDir(), "first.json")
	if err := doc.SaveAs(firstPath); err != nil {
		t.Fatalf("SaveAs(.json): %v", err)
	}
	data, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if trimmed := strings.TrimSpace(string(data)); !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("SaveAs(.json) did not produce JSON: %q", trimmed)
	}

	first, err := Load(firstPath)
	if err != nil {
		t.Fatalf("Load(first.json): %v", err)
	}

	secondPath := filepath.Join(t.TempDir(), "second.json")
	if err := first.SaveAs(secondPath); err != nil {
		t.Fatalf("second SaveAs(.json): %v", err)
	}
	second, err := Load(secondPath)
	if err != nil {
		t.Fatalf("Load(second.json): %v", err)
	}

	if !reflect.DeepEqual(first.Scenario(), second.Scenario()) {
		t.Fatalf("JSON round-trip mismatch:\nfirst:  %#v\nsecond: %#v", first.Scenario(), second.Scenario())
	}
}

func TestSaveClearsDirtyAndSaveAsUpdatesPath(t *testing.T) {
	doc := New()
	doc.MarkDirty()
	if !doc.Dirty() {
		t.Fatal("expected Dirty() true after MarkDirty")
	}

	dst := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := doc.SaveAs(dst); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	if doc.Dirty() {
		t.Fatal("expected Dirty() false after a successful SaveAs")
	}
	if doc.Path() != dst {
		t.Fatalf("Path() = %q, want %q", doc.Path(), dst)
	}

	doc.MarkDirty()
	if err := doc.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if doc.Dirty() {
		t.Fatal("expected Dirty() false after Save")
	}
}
