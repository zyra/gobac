package ui

import (
	"context"
	"net"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
)

// fakeDiscoverSession is a minimal session.Session fake whose Discover
// returns a pre-built channel. The other methods are unused by the
// discovery view and just return zero values.
type fakeDiscoverSession struct {
	ch  chan session.DeviceSummary
	err error
}

var _ session.Session = (*fakeDiscoverSession)(nil)

func (f *fakeDiscoverSession) Start(session.Config) error { return nil }
func (f *fakeDiscoverSession) Stop() error                { return nil }

func (f *fakeDiscoverSession) Discover(ctx context.Context, timeout time.Duration) (<-chan session.DeviceSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ch, nil
}

func (f *fakeDiscoverSession) ReadProperty(ctx context.Context, dev session.Address, obj session.ObjectRef, prop uint32) ([]session.Value, error) {
	return nil, nil
}

func (f *fakeDiscoverSession) ReadMultiple(ctx context.Context, dev session.Address, specs []session.ReadSpec) ([]session.ObjectResult, error) {
	return nil, nil
}

func (f *fakeDiscoverSession) Write(ctx context.Context, dev session.Address, obj session.ObjectRef, w session.WriteRequest) error {
	return nil
}

// fixedLastSeen is the fixed clock value DeviceStore.Now returns in these
// tests, chosen so its "15:04:05" formatting matches its own field values
// for a readable assertion.
var fixedLastSeen = time.Date(2026, 7, 17, 15, 4, 5, 0, time.UTC)

// newDiscoveryTestView builds a DiscoveryView wired to sess and a fresh
// DeviceStore with a fixed clock, returning the concrete type for
// same-package field access.
func newDiscoveryTestView(t *testing.T, sess session.Session) (*DiscoveryView, *store.DeviceStore) {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	t.Cleanup(w.Close)

	shell := NewAppShell(a, w)
	devices := store.NewDeviceStore()
	devices.Now = func() time.Time { return fixedLastSeen }

	obj := NewDiscoveryView(sess, devices, shell)
	view, ok := obj.(*DiscoveryView)
	if !ok {
		t.Fatalf("NewDiscoveryView returned %T, want *DiscoveryView", obj)
	}
	return view, devices
}

// twoTestSummaries returns two DeviceSummary values whose expected discovery
// row rendering is asserted exactly in the tests below.
func twoTestSummaries() []session.DeviceSummary {
	return []session.DeviceSummary{
		{
			Instance:     70001,
			IP:           net.ParseIP("192.0.2.10"),
			Port:         47808,
			VendorID:     260,
			MaxApdu:      1476,
			Segmentation: 3, // none
		},
		{
			Instance:     70002,
			IP:           net.ParseIP("192.0.2.11"),
			Port:         47808,
			VendorID:     10,
			MaxApdu:      480,
			Segmentation: 0, // both
		},
	}
}

// closedChannelOf returns a buffered channel pre-loaded with summaries and
// already closed, so a receiver's `range` drains it and exits immediately
// without any further send/close from the test goroutine (avoids a second
// goroutine racing with the view's sweep goroutine).
func closedChannelOf(summaries []session.DeviceSummary) chan session.DeviceSummary {
	ch := make(chan session.DeviceSummary, len(summaries))
	for _, s := range summaries {
		ch <- s
	}
	close(ch)
	return ch
}

// awaitSweep blocks until done is closed (signaling the view's sweep
// goroutine has fully finished, including re-enabling the sweep button and
// setting shell status) or fails the test after a timeout. This is the
// synchronization point that makes it safe to read view/widget state from
// the test goroutine afterward without racing the background goroutine.
func awaitSweep(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweep did not complete within timeout")
	}
}

// cellText renders the data cell at (row, col) through the table's own
// UpdateCell callback and returns the resulting label text.
func cellText(view *DiscoveryView, row, col int) string {
	label := widget.NewLabel("")
	view.table.UpdateCell(widget.TableCellID{Row: row, Col: col}, label)
	return label.Text
}

func TestSweepPopulatesTableWithExactCellText(t *testing.T) {
	fake := &fakeDiscoverSession{ch: closedChannelOf(twoTestSummaries())}
	view, _ := newDiscoveryTestView(t, fake)
	view.sweepDone = make(chan struct{})

	test.Tap(view.sweepBtn)
	awaitSweep(t, view.sweepDone)

	rows, cols := view.table.Length()
	if rows != 2 {
		t.Fatalf("table rows = %d, want 2", rows)
	}
	if cols != len(discoveryColumns) {
		t.Fatalf("table cols = %d, want %d", cols, len(discoveryColumns))
	}

	want := []string{"70001", "192.0.2.10:47808", "260", "1476", "none", "network", "15:04:05"}
	for col, wantText := range want {
		if got := cellText(view, 0, col); got != wantText {
			t.Errorf("row 0 col %d (%s) = %q, want %q", col, discoveryColumns[col], got, wantText)
		}
	}
}

func TestSweepSetsExactDeviceCountLabel(t *testing.T) {
	fake := &fakeDiscoverSession{ch: closedChannelOf(twoTestSummaries())}
	view, _ := newDiscoveryTestView(t, fake)
	view.sweepDone = make(chan struct{})

	test.Tap(view.sweepBtn)
	awaitSweep(t, view.sweepDone)

	if got, want := view.count.Text, "2 devices"; got != want {
		t.Errorf("count label = %q, want %q", got, want)
	}
}

func TestClearButtonEmptiesTableAndCount(t *testing.T) {
	fake := &fakeDiscoverSession{ch: closedChannelOf(twoTestSummaries())}
	view, _ := newDiscoveryTestView(t, fake)
	view.sweepDone = make(chan struct{})

	test.Tap(view.sweepBtn)
	awaitSweep(t, view.sweepDone)

	if got, want := view.count.Text, "2 devices"; got != want {
		t.Fatalf("precondition: count label = %q, want %q", got, want)
	}

	test.Tap(view.clearBtn)

	if got, want := view.count.Text, "0 devices"; got != want {
		t.Errorf("count label after clear = %q, want %q", got, want)
	}
	if rows, _ := view.table.Length(); rows != 0 {
		t.Errorf("table rows after clear = %d, want 0", rows)
	}
}

func TestSweepButtonDisabledWhileRunningAndReenabledAfter(t *testing.T) {
	ch := make(chan session.DeviceSummary) // unbuffered, left open
	fake := &fakeDiscoverSession{ch: ch}
	view, _ := newDiscoveryTestView(t, fake)
	view.sweepDone = make(chan struct{})

	test.Tap(view.sweepBtn)

	// The sweep goroutine, if it has started at all, is parked on the
	// still-open, empty channel and has not touched sweepBtn again since
	// Disable() ran synchronously above — safe to read here.
	if !view.sweepBtn.Disabled() {
		t.Fatal("sweep button should be disabled while a sweep is running")
	}

	close(ch)
	awaitSweep(t, view.sweepDone)

	if view.sweepBtn.Disabled() {
		t.Error("sweep button should be re-enabled after the sweep completes")
	}
}

func TestSelectingRowInvokesOnSelectWithExactRow(t *testing.T) {
	fake := &fakeDiscoverSession{}
	view, devices := newDiscoveryTestView(t, fake)

	rowA := store.DeviceRow{
		Key:          store.DeviceKey{Instance: 1, IP: "192.0.2.1"},
		Port:         47808,
		VendorID:     5,
		MaxApdu:      480,
		Segmentation: 0,
		Source:       "network",
	}
	rowB := store.DeviceRow{
		Key:          store.DeviceKey{Instance: 2, IP: "192.0.2.2"},
		Port:         47808,
		VendorID:     6,
		MaxApdu:      480,
		Segmentation: 1,
		Source:       "network",
	}
	devices.Upsert(rowA)
	devices.Upsert(rowB)

	var calls int
	var got store.DeviceRow
	view.OnSelect = func(row store.DeviceRow) {
		calls++
		got = row
	}

	// Snapshot is sorted by Instance ascending: row index 0 is rowA
	// (Instance 1), row index 1 is rowB (Instance 2).
	view.table.Select(widget.TableCellID{Row: 1, Col: 0})

	if calls != 1 {
		t.Fatalf("OnSelect called %d times, want 1", calls)
	}
	want := rowB
	want.LastSeen = fixedLastSeen
	if got != want {
		t.Errorf("OnSelect row = %+v, want %+v", got, want)
	}
}
