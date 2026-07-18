package ui

import (
	"bytes"
	"context"
	"image"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/assets"
	"github.com/zyra/gobac/gui/internal/scenariodoc"
	"github.com/zyra/gobac/gui/internal/simrun"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/v2/simulator"
)

// newEditorTestView builds an EditorView inside a headless test app/window
// (dialog.NewFileOpen/NewFileSave need a window to attach to, even though
// these tests never open them), returning the concrete type for
// same-package field access and the DeviceStore it was wired to.
func newEditorTestView(t *testing.T) (*EditorView, *store.DeviceStore) {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	t.Cleanup(w.Close)

	shell := NewAppShell(a, w)
	devices := store.NewDeviceStore()
	obj := NewEditorView(devices, shell)
	view, ok := obj.(*EditorView)
	if !ok {
		t.Fatalf("NewEditorView returned %T, want *EditorView", obj)
	}
	return view, devices
}

// TestNewDocumentHasOneDeviceAndValidates covers the brief's "New -> Add
// Device" case: scenariodoc.New() (what NewEditorView starts from) already
// seeds one device, so a freshly constructed view's device list already
// shows exactly that device and the document already validates.
func TestNewDocumentHasOneDeviceAndValidates(t *testing.T) {
	view, _ := newEditorTestView(t)

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
	view, _ := newEditorTestView(t)

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
	view, _ := newEditorTestView(t)
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
	view, _ := newEditorTestView(t)

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
	view, _ := newEditorTestView(t)
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

// ---- Run / Stop (task U3, migrated from the deleted quickstart_test.go) ----

// fakeSimRunner is a minimal simRunner fake: Devices returns a fixed set,
// Stop just records that it was called, and Err never fires.
type fakeSimRunner struct {
	devices []simrun.RunningDevice
	stopped bool
	errCh   chan error
}

func newFakeSimRunner() *fakeSimRunner {
	return &fakeSimRunner{
		devices: []simrun.RunningDevice{
			{ID: 1001, Name: "Boiler", Addr: "127.0.0.2", Port: 47901},
			{ID: 1002, Name: "AHU", Addr: "127.0.0.2", Port: 47902},
		},
		errCh: make(chan error),
	}
}

func (f *fakeSimRunner) Devices() []simrun.RunningDevice { return f.devices }
func (f *fakeSimRunner) Stop()                           { f.stopped = true }
func (f *fakeSimRunner) Err() <-chan error               { return f.errCh }

// awaitEditor blocks until done is closed (signaling the view's Run/Stop
// background goroutine has fully finished) or fails the test after a
// timeout, mirroring awaitSweep/awaitQuickstart's synchronization pattern.
func awaitEditor(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run/stop operation did not complete within timeout")
	}
}

// lookupEditorRow finds the store row for (instance, ip), if any.
func lookupEditorRow(devices *store.DeviceStore, instance uint32, ip string) (store.DeviceRow, bool) {
	for _, row := range devices.Snapshot() {
		if row.Key.Instance == instance && row.Key.IP == ip {
			return row, true
		}
	}
	return store.DeviceRow{}, false
}

// TestRunPopulatesDeviceStoreAndRunningStrip taps the rendered Run button
// and asserts DeviceStore rows appear with Source == "simulated" and Name
// set, the running-devices strip becomes visible, and the Stop button is
// enabled while Run stays disabled.
func TestRunPopulatesDeviceStoreAndRunningStrip(t *testing.T) {
	view, devices := newEditorTestView(t)
	fake := newFakeSimRunner()
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (simRunner, error) {
		return fake, nil
	}
	view.startDone = make(chan struct{})

	test.Tap(view.runBtn)
	awaitEditor(t, view.startDone)

	if got, want := devices.Len(), 2; got != want {
		t.Fatalf("devices.Len() = %d, want %d", got, want)
	}
	for _, d := range fake.devices {
		row, ok := lookupEditorRow(devices, d.ID, d.Addr)
		if !ok {
			t.Fatalf("store missing row for device %d/%s", d.ID, d.Addr)
		}
		if row.Source != "simulated" {
			t.Errorf("device %d Source = %q, want %q", d.ID, row.Source, "simulated")
		}
		if row.Name != d.Name {
			t.Errorf("device %d Name = %q, want %q", d.ID, row.Name, d.Name)
		}
		if row.Port != d.Port {
			t.Errorf("device %d Port = %d, want %d", d.ID, row.Port, d.Port)
		}
	}

	if view.stopBtn.Disabled() {
		t.Error("stop button should be enabled after a successful run")
	}
	if !view.runBtn.Disabled() {
		t.Error("run button should stay disabled while the simulation is running")
	}
	if !view.runningStrip.Visible() {
		t.Error("running-devices strip should be visible while a simulation is running")
	}
	if got, want := len(view.runningRowsBox.Objects), 2; got != want {
		t.Errorf("len(runningRowsBox.Objects) = %d, want %d", got, want)
	}
	for i, want := range []string{
		"1001 Boiler — 127.0.0.2:47901",
		"1002 AHU — 127.0.0.2:47902",
	} {
		lbl, ok := view.runningRowsBox.Objects[i].(*widget.Label)
		if !ok {
			t.Fatalf("runningRowsBox.Objects[%d] = %T, want *widget.Label", i, view.runningRowsBox.Objects[i])
		}
		if lbl.Text != want {
			t.Errorf("runningRowsBox.Objects[%d].Text = %q, want %q", i, lbl.Text, want)
		}
	}

	got := view.shell.Status.Text
	if !strings.HasPrefix(got, "Simulation running — 2 devices (ports ") {
		t.Errorf("status = %q, want prefix %q", got, "Simulation running — 2 devices (ports ")
	}
}

// TestRunMakesRunningStripVisibleOnCanvas mounts the editor view into a real
// window/canvas (unlike TestRunPopulatesDeviceStoreAndRunningStrip, which
// only inspects Visible()/Objects without ever rendering) and asserts the
// running-devices strip actually occupies rendered space and paints content
// once a simulation starts. This is the regression test for the
// v.root.Refresh() fix in onRun/onStop: runningStrip.Show() alone leaves the
// Border layout's bottom region sized from when the strip was hidden
// (zero), so without re-running that layout the strip would report
// Visible() == true while remaining invisibly clipped to zero size — a
// capture-only test catches that; a Visible()/Objects-only test does not.
func TestRunMakesRunningStripVisibleOnCanvas(t *testing.T) {
	view, _ := newEditorTestView(t)
	fake := newFakeSimRunner()
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (simRunner, error) {
		return fake, nil
	}
	view.startDone = make(chan struct{})

	a := test.NewApp()
	w := a.NewWindow("capture")
	defer w.Close()
	w.SetContent(view)
	w.Resize(fyne.NewSize(900, 700))

	before := w.Canvas().Capture()

	test.Tap(view.runBtn)
	awaitEditor(t, view.startDone)

	after := w.Canvas().Capture()
	if imagesEqual(before, after) {
		t.Fatal("canvas capture is unchanged after Run; running-devices strip never became visible")
	}

	stripSize := view.runningStrip.Size()
	if stripSize.Width <= 0 || stripSize.Height <= 0 {
		t.Fatalf("runningStrip.Size() = %v, want > 0 in both dimensions (strip must occupy real layout space once visible, not just report Visible() == true)", stripSize)
	}

	stripPos := view.runningStrip.Position()
	region := image.Rect(
		int(stripPos.X), int(stripPos.Y),
		int(stripPos.X+stripSize.Width), int(stripPos.Y+stripSize.Height),
	)
	if got := distinctColorsInRegion(after, region); got <= 1 {
		t.Fatalf("running-devices strip region has %d distinct color(s), want > 1 (region should render device rows, not blank/clipped content)", got)
	}
}

// TestRunAppendsPortHintToStatus covers the boot.go-wired PortHint seam: a
// non-empty return value is appended to the running-status text, and the
// callback receives every running device's port.
func TestRunAppendsPortHintToStatus(t *testing.T) {
	view, _ := newEditorTestView(t)
	fake := newFakeSimRunner()
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (simRunner, error) {
		return fake, nil
	}
	var gotPorts []uint16
	view.PortHint = func(ports []uint16) string {
		gotPorts = ports
		return "Tip: set Settings → Port to 47901 to interact with these devices."
	}
	view.startDone = make(chan struct{})

	test.Tap(view.runBtn)
	awaitEditor(t, view.startDone)

	want := "Simulation running — 2 devices (ports 47901, 47902) Tip: set Settings → Port to 47901 to interact with these devices."
	if got := view.shell.Status.Text; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	wantPorts := []uint16{47901, 47902}
	if len(gotPorts) != len(wantPorts) {
		t.Fatalf("PortHint received %d ports, want %d", len(gotPorts), len(wantPorts))
	}
	for i, p := range wantPorts {
		if gotPorts[i] != p {
			t.Errorf("PortHint ports[%d] = %d, want %d", i, gotPorts[i], p)
		}
	}
}

// TestStopRemovesInjectedRowsAndHidesStrip taps Run then Stop and asserts
// the injected rows are removed, the runner's Stop was called, the strip
// hides again, and the buttons reset.
func TestStopRemovesInjectedRowsAndHidesStrip(t *testing.T) {
	view, devices := newEditorTestView(t)
	fake := newFakeSimRunner()
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (simRunner, error) {
		return fake, nil
	}
	view.startDone = make(chan struct{})

	test.Tap(view.runBtn)
	awaitEditor(t, view.startDone)

	if got, want := devices.Len(), 2; got != want {
		t.Fatalf("precondition: devices.Len() = %d, want %d", got, want)
	}

	view.stopDone = make(chan struct{})
	test.Tap(view.stopBtn)
	awaitEditor(t, view.stopDone)

	if !fake.stopped {
		t.Error("expected the runner's Stop to have been called")
	}
	if got, want := devices.Len(), 0; got != want {
		t.Errorf("devices.Len() after stop = %d, want %d", got, want)
	}
	if view.runningStrip.Visible() {
		t.Error("running-devices strip should hide after Stop")
	}
	if got, want := len(view.runningRowsBox.Objects), 0; got != want {
		t.Errorf("len(runningRowsBox.Objects) after stop = %d, want %d", got, want)
	}
	if view.runBtn.Disabled() {
		t.Error("run button should be re-enabled after stop")
	}
	if !view.stopBtn.Disabled() {
		t.Error("stop button should be disabled after stop")
	}
	if got, want := view.shell.Status.Text, "Simulation stopped"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

// TestRunRejectsNonLoopbackScenarioWithPlainLanguageMessage covers the
// simrun.ErrUnsupportedScenario path: Run never starts the runner (no
// devices injected) and the status bar shows plain-language guidance
// rather than the raw error text.
func TestRunRejectsNonLoopbackScenarioWithPlainLanguageMessage(t *testing.T) {
	view, devices := newEditorTestView(t)
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (simRunner, error) {
		return nil, simrun.ErrUnsupportedScenario
	}
	view.startDone = make(chan struct{})

	test.Tap(view.runBtn)
	awaitEditor(t, view.startDone)

	want := "Simulations run privately on this computer. Set the network mode to multi-port with loopback addresses (or use the example scenario)."
	if got := view.shell.Status.Text; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	if view.running != nil {
		t.Error("view.running should be nil after a rejected Run")
	}
	if devices.Len() != 0 {
		t.Errorf("devices.Len() = %d, want 0 (no devices injected on rejection)", devices.Len())
	}
	if view.runBtn.Disabled() {
		t.Error("run button should be re-enabled after a rejected Run")
	}
	if !view.stopBtn.Disabled() {
		t.Error("stop button should stay disabled after a rejected Run")
	}
}

// TestLoadExampleScenarioReplacesDeviceList taps "Load example scenario" on
// a fresh (non-dirty) document and asserts the editor's device list renders
// the bundled example's devices: the rendered capture changes and the
// device count matches the example scenario's device count exactly.
func TestLoadExampleScenarioReplacesDeviceList(t *testing.T) {
	view, _ := newEditorTestView(t)

	a := test.NewApp()
	w := a.NewWindow("capture")
	defer w.Close()
	w.SetContent(view)
	w.Resize(fyne.NewSize(900, 600))

	before := w.Canvas().Capture()

	test.Tap(view.loadExampleBtn)

	after := w.Canvas().Capture()
	if imagesEqual(before, after) {
		t.Fatal("canvas capture is unchanged after tapping Load example scenario")
	}

	exampleScenario, err := simulator.DecodeScenario(bytes.NewReader(assets.QuickstartScenario), "yaml")
	if err != nil {
		t.Fatalf("decode bundled example scenario: %v", err)
	}
	want := len(exampleScenario.Devices)
	if got := view.deviceListLength(); got != want {
		t.Errorf("deviceListLength() after Load example scenario = %d, want %d (example device count)", got, want)
	}
	if got, want := view.summaryLabel.Text, "valid"; got != want {
		t.Errorf("summaryLabel.Text = %q, want %q", got, want)
	}
}

// distinctColorsInRegion returns the number of distinct pixel colors within
// the intersection of img's bounds and region.
func distinctColorsInRegion(img image.Image, region image.Rectangle) int {
	b := img.Bounds().Intersect(region)
	seen := make(map[[4]uint32]struct{})
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bch, aCh := img.At(x, y).RGBA()
			seen[[4]uint32{r, g, bch, aCh}] = struct{}{}
		}
	}
	return len(seen)
}
