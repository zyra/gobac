package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/simrun"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/v2/simulator"
)

// fakeQuickstartRunner is a minimal quickstartRunner fake: Devices returns a
// fixed set, Stop just records that it was called, and Err never fires.
type fakeQuickstartRunner struct {
	devices []simrun.RunningDevice
	stopped bool
	errCh   chan error
}

func newFakeQuickstartRunner() *fakeQuickstartRunner {
	return &fakeQuickstartRunner{
		devices: []simrun.RunningDevice{
			{ID: 1001, Name: "Boiler", Addr: "127.0.0.2", Port: 47901},
			{ID: 1002, Name: "AHU", Addr: "127.0.0.2", Port: 47902},
			{ID: 1003, Name: "Lab Sensor", Addr: "127.0.0.2", Port: 47903},
		},
		errCh: make(chan error),
	}
}

func (f *fakeQuickstartRunner) Devices() []simrun.RunningDevice { return f.devices }
func (f *fakeQuickstartRunner) Stop()                           { f.stopped = true }
func (f *fakeQuickstartRunner) Err() <-chan error               { return f.errCh }

// newQuickstartTestView builds a QuickstartView wired to a fresh DeviceStore
// and a fake StartRunner, returning the concrete type for same-package field
// access.
func newQuickstartTestView(t *testing.T, fake *fakeQuickstartRunner, startErr error) (*QuickstartView, *store.DeviceStore) {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	t.Cleanup(w.Close)

	shell := NewAppShell(a, w)
	devices := store.NewDeviceStore()

	obj := NewQuickstartView(devices, shell)
	view, ok := obj.(*QuickstartView)
	if !ok {
		t.Fatalf("NewQuickstartView returned %T, want *QuickstartView", obj)
	}
	view.StartRunner = func(ctx context.Context, sc *simulator.Scenario) (quickstartRunner, error) {
		if startErr != nil {
			return nil, startErr
		}
		return fake, nil
	}
	return view, devices
}

func awaitQuickstart(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		// Generous for slow CI runners (see awaitBrowser in
		// browser_test.go); a passing wait returns immediately.
		t.Fatal("quickstart operation did not complete within timeout")
	}
}

func listText(view *QuickstartView, id widget.ListItemID) string {
	label := widget.NewLabel("")
	view.list.UpdateItem(id, label)
	return label.Text
}

func TestStartPopulatesDeviceListAndStore(t *testing.T) {
	fake := newFakeQuickstartRunner()
	view, devices := newQuickstartTestView(t, fake, nil)
	view.startDone = make(chan struct{})

	test.Tap(view.startBtn)
	awaitQuickstart(t, view.startDone)

	if got := view.list.Length(); got != 3 {
		t.Fatalf("list length = %d, want 3", got)
	}
	want := []string{
		"1001 Boiler — 127.0.0.2:47901",
		"1002 AHU — 127.0.0.2:47902",
		"1003 Lab Sensor — 127.0.0.2:47903",
	}
	for i, wantText := range want {
		if got := listText(view, widget.ListItemID(i)); got != wantText {
			t.Errorf("row %d = %q, want %q", i, got, wantText)
		}
	}

	if got, want := devices.Len(), 3; got != want {
		t.Fatalf("devices.Len() = %d, want %d", got, want)
	}
	for _, d := range fake.devices {
		row, ok := lookupRow(devices, d.ID, d.Addr)
		if !ok {
			t.Fatalf("store missing row for device %d/%s", d.ID, d.Addr)
		}
		if row.Source != "local-sim" {
			t.Errorf("device %d Source = %q, want %q", d.ID, row.Source, "local-sim")
		}
		if row.Port != d.Port {
			t.Errorf("device %d Port = %d, want %d", d.ID, row.Port, d.Port)
		}
	}

	if view.stopBtn.Disabled() {
		t.Error("stop button should be enabled after a successful start")
	}
	if !view.startBtn.Disabled() {
		t.Error("start button should stay disabled while the simulator is running")
	}
}

func TestStopRemovesInjectedRowsAndResetsButtons(t *testing.T) {
	fake := newFakeQuickstartRunner()
	view, devices := newQuickstartTestView(t, fake, nil)
	view.startDone = make(chan struct{})

	test.Tap(view.startBtn)
	awaitQuickstart(t, view.startDone)

	if got, want := devices.Len(), 3; got != want {
		t.Fatalf("precondition: devices.Len() = %d, want %d", got, want)
	}

	view.stopDone = make(chan struct{})
	test.Tap(view.stopBtn)
	awaitQuickstart(t, view.stopDone)

	if !fake.stopped {
		t.Error("expected the runner's Stop to have been called")
	}
	if got, want := devices.Len(), 0; got != want {
		t.Errorf("devices.Len() after stop = %d, want %d", got, want)
	}
	if got := view.list.Length(); got != 0 {
		t.Errorf("list length after stop = %d, want 0", got)
	}
	if view.startBtn.Disabled() {
		t.Error("start button should be re-enabled after stop")
	}
	if !view.stopBtn.Disabled() {
		t.Error("stop button should be disabled after stop")
	}
}

// lookupRow finds the store row for (instance, ip), if any.
func lookupRow(devices *store.DeviceStore, instance uint32, ip string) (store.DeviceRow, bool) {
	for _, row := range devices.Snapshot() {
		if row.Key.Instance == instance && row.Key.IP == ip {
			return row, true
		}
	}
	return store.DeviceRow{}, false
}
