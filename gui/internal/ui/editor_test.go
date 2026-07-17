package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/zyra/gobac/gui/internal/scenariodoc"
)

// newEditorTestView builds an EditorView inside a headless test app/window
// (dialog.NewFileOpen/NewFileSave need a window to attach to, even though
// these tests never open them), returning the concrete type for
// same-package field access.
func newEditorTestView(t *testing.T) *EditorView {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	t.Cleanup(w.Close)

	shell := NewAppShell(a, w)
	obj := NewEditorView(shell)
	view, ok := obj.(*EditorView)
	if !ok {
		t.Fatalf("NewEditorView returned %T, want *EditorView", obj)
	}
	return view
}

// TestNewDocumentHasOneDeviceAndValidates covers the brief's "New -> Add
// Device" case: scenariodoc.New() (what NewEditorView starts from) already
// seeds one device, so a freshly constructed view's device list already
// shows exactly that device and the document already validates.
func TestNewDocumentHasOneDeviceAndValidates(t *testing.T) {
	view := newEditorTestView(t)

	if got, want := view.deviceListLength(), 1; got != want {
		t.Fatalf("deviceListLength() = %d, want %d", got, want)
	}
	if got, want := view.deviceCellText(0), "1 — device-1"; got != want {
		t.Errorf("deviceCellText(0) = %q, want %q", got, want)
	}
	if got, want := view.summaryLabel.Text, "valid"; got != want {
		t.Errorf("summaryLabel.Text = %q, want %q", got, want)
	}
}

// TestAddAnalogValueObjectSetsExactFields drives the type picker, Add
// button, Present Value entry, and Writable check, then asserts the
// resulting simulator.ObjectSpec fields directly.
func TestAddAnalogValueObjectSetsExactFields(t *testing.T) {
	view := newEditorTestView(t)

	view.objTypeSelect.SetSelected("analog-value")
	view.addObjectBtn.OnTapped()

	if view.objPresentValueEntry == nil {
		t.Fatal("objPresentValueEntry is nil after adding an analog-value object")
	}
	view.objPresentValueEntry.SetText("21.5")
	view.objWritableCheck.SetChecked(true)

	obj := view.doc.Scenario().Devices[0].Objects[0]
	if got, want := obj.Type, "analog-value"; got != want {
		t.Errorf("obj.Type = %q, want %q", got, want)
	}
	if got, want := obj.Instance, uint32(1); got != want {
		t.Errorf("obj.Instance = %d, want %d", got, want)
	}
	pv, ok := obj.PresentValue.(float64)
	if !ok {
		t.Fatalf("obj.PresentValue = %#v (%T), want float64", obj.PresentValue, obj.PresentValue)
	}
	if float32(pv) != 21.5 {
		t.Errorf("obj.PresentValue as float32 = %v, want 21.5", float32(pv))
	}
	if !obj.Writable {
		t.Error("obj.Writable = false, want true")
	}
}

// TestCommandableForcesWritableAndInitialPriorityValidates covers the
// Commandable -> Writable auto-check/disable behavior and the initial
// priority 6 (reserved) / 8 (valid) field-error + Save-button transitions.
func TestCommandableForcesWritableAndInitialPriorityValidates(t *testing.T) {
	view := newEditorTestView(t)
	view.objTypeSelect.SetSelected("analog-value")
	view.addObjectBtn.OnTapped()
	view.objPresentValueEntry.SetText("21.5")

	view.objCommandableCheck.SetChecked(true)

	if !view.objWritableCheck.Checked {
		t.Error("Writable checkbox not checked after Commandable was checked")
	}
	if !view.objWritableCheck.Disabled() {
		t.Error("Writable checkbox not disabled after Commandable was checked")
	}

	view.objInitialPriorityEntry.SetText("6")

	fieldErr, ok := view.fieldErrors["devices[0].objects[0].initial_priority"]
	if !ok {
		t.Fatal("expected a field error for initial_priority = 6")
	}
	if !strings.Contains(fieldErr, "6") {
		t.Errorf("initial_priority field error = %q, want it to contain %q", fieldErr, "6")
	}
	if !view.saveBtn.Disabled() {
		t.Error("Save button not disabled while initial_priority = 6 is invalid")
	}

	view.objInitialPriorityEntry.SetText("8")

	if _, ok := view.fieldErrors["devices[0].objects[0].initial_priority"]; ok {
		t.Error("expected no field error for initial_priority = 8")
	}
	if got, want := view.summaryLabel.Text, "valid"; got != want {
		t.Errorf("summaryLabel.Text = %q, want %q", got, want)
	}
	if view.saveBtn.Disabled() {
		t.Error("Save button disabled despite a valid document")
	}
}

// TestMultiIPModeRequiresAddressOnEmptyDevice covers the multi-ip
// Address-required field error and its clearing on switching back to
// single-device.
func TestMultiIPModeRequiresAddressOnEmptyDevice(t *testing.T) {
	view := newEditorTestView(t)

	view.modeSelect.SetSelected("multi-ip")

	fieldErr, ok := view.fieldErrors["devices[0].address"]
	if !ok {
		t.Fatal("expected a field error for a blank address in multi-ip mode")
	}
	if fieldErr == "" {
		t.Error("address field error is empty")
	}

	view.modeSelect.SetSelected("single-device")

	if _, ok := view.fieldErrors["devices[0].address"]; ok {
		t.Error("expected the address field error to clear after switching back to single-device")
	}
}

// TestSaveAsRoundTripsAndClearsDirty covers Save As: the file exists, a
// fresh Load of it DeepEquals the document's scenario, and the dirty
// marker (title suffix) clears. It opens testdata/scenario.yaml first
// (rather than starting from New()) so every field is already
// defaults-normalized by the initial Load, the same way the manual
// verification flow in the G7 brief does (open, edit a present_value,
// save) — a bare New() document's zero-valued fields would not survive a
// save/reload DeepEqual, since decoding always fills in defaults
// (simulator.applyScenarioDefaults) that a never-loaded document never
// had explicitly set.
func TestSaveAsRoundTripsAndClearsDirty(t *testing.T) {
	view := newEditorTestView(t)
	if err := view.openPath("testdata/scenario.yaml"); err != nil {
		t.Fatalf("openPath(testdata/scenario.yaml): %v", err)
	}

	view.deviceList.Select(0)
	view.objectList.Select(0)
	if view.objPresentValueEntry == nil {
		t.Fatal("objPresentValueEntry is nil after selecting the first object")
	}
	view.objPresentValueEntry.SetText("99.5")

	dst := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := view.saveAs(dst); err != nil {
		t.Fatalf("saveAs: %v", err)
	}

	reloaded, err := scenariodoc.Load(dst)
	if err != nil {
		t.Fatalf("Load(%q): %v", dst, err)
	}
	if !reflect.DeepEqual(view.doc.Scenario(), reloaded.Scenario()) {
		t.Fatalf("round-trip mismatch:\nsaved:    %#v\nreloaded: %#v", view.doc.Scenario(), reloaded.Scenario())
	}

	if view.doc.Dirty() {
		t.Error("doc.Dirty() = true after a successful saveAs")
	}
	if strings.Contains(view.titleLabel.Text, " *") {
		t.Errorf("titleLabel.Text = %q, want no dirty marker", view.titleLabel.Text)
	}
}

func TestClassifyObjectType(t *testing.T) {
	cases := map[string]string{
		"analog-input":      "analog",
		"analog-output":     "analog",
		"analog-value":      "analog",
		"binary-input":      "binary",
		"binary-output":     "binary",
		"binary-value":      "binary",
		"multi-state-input": "multistate",
		"multistate-output": "multistate",
		"multi-state-value": "multistate",
		"not-a-type":        "",
	}
	for in, want := range cases {
		if got := classifyObjectType(in); got != want {
			t.Errorf("classifyObjectType(%q) = %q, want %q", in, got, want)
		}
	}
}
