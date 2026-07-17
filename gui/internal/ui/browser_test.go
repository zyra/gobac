package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
)

// fakeBrowserSession is a minimal session.Session fake driving the object
// browser: ReadProperty answers the Object_List read in LoadDevice,
// ReadMultiple answers the property-panel loads with a canned result set
// (regardless of which object was requested — these tests only ever select
// one object), and Write records its calls.
type fakeBrowserSession struct {
	objectListValues []session.Value
	objectListErr    error

	propertyResults []session.ObjectResult
	propertyErr     error

	writeCalls []fakeWriteCall
	writeErr   error
}

type fakeWriteCall struct {
	dev session.Address
	obj session.ObjectRef
	req session.WriteRequest
}

var _ session.Session = (*fakeBrowserSession)(nil)

func (f *fakeBrowserSession) Start(session.Config) error { return nil }
func (f *fakeBrowserSession) Stop() error                { return nil }

func (f *fakeBrowserSession) Discover(ctx context.Context, timeout time.Duration) (<-chan session.DeviceSummary, error) {
	ch := make(chan session.DeviceSummary)
	close(ch)
	return ch, nil
}

func (f *fakeBrowserSession) ReadProperty(ctx context.Context, dev session.Address, obj session.ObjectRef, prop uint32) ([]session.Value, error) {
	if f.objectListErr != nil {
		return nil, f.objectListErr
	}
	return f.objectListValues, nil
}

func (f *fakeBrowserSession) ReadMultiple(ctx context.Context, dev session.Address, specs []session.ReadSpec) ([]session.ObjectResult, error) {
	if f.propertyErr != nil {
		return nil, f.propertyErr
	}
	return f.propertyResults, nil
}

func (f *fakeBrowserSession) Write(ctx context.Context, dev session.Address, obj session.ObjectRef, w session.WriteRequest) error {
	f.writeCalls = append(f.writeCalls, fakeWriteCall{dev: dev, obj: obj, req: w})
	return f.writeErr
}

// twoObjectList is the Object_List reply used by the tests below: an
// analog-value instance 1 and a binary-value instance 3.
func twoObjectList() []session.Value {
	return []session.Value{
		{Tag: 12, Value: session.ObjectRef{Type: 2, Instance: 1}},
		{Tag: 12, Value: session.ObjectRef{Type: 5, Instance: 3}},
	}
}

// cannedPropertyResults returns one ObjectResult per wave1PropertyIDs
// entry (readProperties issues one ReadSpec per property, in that fixed
// order): Present_Value (index 2, id 85) succeeds with a Real 42.5; Units
// (index 3, id 117) fails with "unknown-property"; every other property
// succeeds with no values.
func cannedPropertyResults() []session.ObjectResult {
	results := make([]session.ObjectResult, len(wave1PropertyIDs))
	for i, id := range wave1PropertyIDs {
		switch id {
		case 85: // Present_Value
			results[i] = session.ObjectResult{
				Values: []session.Value{{Tag: 4, Value: float32(42.5)}},
			}
		case 117: // Units
			results[i] = session.ObjectResult{
				Errors: map[uint32]error{117: errors.New("unknown-property")},
			}
		default:
			results[i] = session.ObjectResult{}
		}
	}
	return results
}

// newBrowserTestView builds a BrowserView wired to fake and a fresh
// ObjectCache, returning the concrete type for same-package field access.
func newBrowserTestView(t *testing.T, fake *fakeBrowserSession) *BrowserView {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	t.Cleanup(w.Close)

	shell := NewAppShell(a, w)
	objects := store.NewObjectCache()

	obj := NewBrowserView(fake, objects, shell)
	view, ok := obj.(*BrowserView)
	if !ok {
		t.Fatalf("NewBrowserView returned %T, want *BrowserView", obj)
	}
	return view
}

func awaitBrowser(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("background goroutine did not complete within timeout")
	}
}

// groupLabels returns the exact labels the tree renders for its root
// (group) nodes, in the order childUIDs("") returns them.
func groupLabels(view *BrowserView) []string {
	uids := view.childUIDs("")
	labels := make([]string, len(uids))
	for i, uid := range uids {
		label := widget.NewLabel("")
		view.updateTreeNode(uid, true, label)
		labels[i] = label.Text
	}
	return labels
}

func TestLoadDevicePopulatesTreeWithExactGroupLabels(t *testing.T) {
	fake := &fakeBrowserSession{objectListValues: twoObjectList()}
	view := newBrowserTestView(t, fake)
	view.loadDone = make(chan struct{})

	view.LoadDevice(store.DeviceRow{Key: store.DeviceKey{Instance: 10, IP: "192.0.2.1"}})
	awaitBrowser(t, view.loadDone)

	labels := groupLabels(view)
	want := []string{"analog-value", "binary-value"}
	if len(labels) != len(want) {
		t.Fatalf("group labels = %v, want %v", labels, want)
	}
	for i, w := range want {
		if labels[i] != w {
			t.Errorf("group label %d = %q, want %q", i, labels[i], w)
		}
	}
}

func TestSelectingLeafFillsPropertyTableExactCells(t *testing.T) {
	fake := &fakeBrowserSession{
		objectListValues: twoObjectList(),
		propertyResults:  cannedPropertyResults(),
	}
	view := newBrowserTestView(t, fake)
	view.loadDone = make(chan struct{})
	view.LoadDevice(store.DeviceRow{Key: store.DeviceKey{Instance: 10, IP: "192.0.2.1"}})
	awaitBrowser(t, view.loadDone)

	view.propsDone = make(chan struct{})
	view.tree.Select(leafUID(2, 1)) // "analog-value 1"
	awaitBrowser(t, view.propsDone)

	rows, cols := view.table.Length()
	if rows != len(wave1PropertyIDs) {
		t.Fatalf("table rows = %d, want %d", rows, len(wave1PropertyIDs))
	}
	if cols != len(propertyColumns) {
		t.Fatalf("table cols = %d, want %d", cols, len(propertyColumns))
	}

	presentValueRow := -1
	unitsRow := -1
	for i, id := range wave1PropertyIDs {
		switch id {
		case 85:
			presentValueRow = i
		case 117:
			unitsRow = i
		}
	}

	if got, want := browserCellText(view, presentValueRow, 0), "Present_Value"; got != want {
		t.Errorf("Present_Value row property cell = %q, want %q", got, want)
	}
	if got, want := browserCellText(view, presentValueRow, 1), "42.5"; got != want {
		t.Errorf("Present_Value row value cell = %q, want %q", got, want)
	}
	if got, want := browserCellText(view, presentValueRow, 2), ""; got != want {
		t.Errorf("Present_Value row error cell = %q, want %q", got, want)
	}

	if got, want := browserCellText(view, unitsRow, 2), "unknown-property"; got != want {
		t.Errorf("Units row error cell = %q, want %q", got, want)
	}
}

// browserCellText renders the data cell at (row, col) through the table's own
// UpdateCell callback and returns the resulting label text.
func browserCellText(view *BrowserView, row, col int) string {
	label := widget.NewLabel("")
	view.table.UpdateCell(widget.TableCellID{Row: row, Col: col}, label)
	return label.Text
}

func TestValidateWritePriorityRejectsReservedValue(t *testing.T) {
	err := validateWritePriority("6")
	if err == nil {
		t.Fatal("validateWritePriority(\"6\") = nil, want an error")
	}
	if got, want := err.Error(), "priority 6 is reserved"; got != want {
		t.Errorf("validateWritePriority(\"6\") error = %q, want %q", got, want)
	}
}

func TestValidateWritePriorityAcceptsInRangeAndNone(t *testing.T) {
	for _, text := range []string{"0", "", "1", "16"} {
		if err := validateWritePriority(text); err != nil {
			t.Errorf("validateWritePriority(%q) = %v, want nil", text, err)
		}
	}
}

func TestSubmitWriteCallsFakeWriteWithExactRequest(t *testing.T) {
	fake := &fakeBrowserSession{
		objectListValues: twoObjectList(),
		propertyResults:  cannedPropertyResults(),
	}
	view := newBrowserTestView(t, fake)
	view.loadDone = make(chan struct{})
	view.LoadDevice(store.DeviceRow{Key: store.DeviceKey{Instance: 10, IP: "192.0.2.1"}})
	awaitBrowser(t, view.loadDone)

	view.propsDone = make(chan struct{})
	view.tree.Select(leafUID(2, 1)) // selects analog-value 1
	awaitBrowser(t, view.propsDone)

	view.openWriteDialog()
	view.writeValueEntry.SetText("20")
	view.writeTagSelect.SetSelected("Real(4)")
	view.writePriorityEntry.SetText("8")

	view.writeDone = make(chan struct{})
	view.submitWrite()
	awaitBrowser(t, view.writeDone)

	if len(fake.writeCalls) != 1 {
		t.Fatalf("Write called %d times, want 1", len(fake.writeCalls))
	}
	call := fake.writeCalls[0]

	wantAddr := session.Address{IP: view.deviceAddr.IP}
	if call.dev.IP.String() != wantAddr.IP.String() {
		t.Errorf("Write dev = %+v, want %+v", call.dev, wantAddr)
	}
	wantObj := session.ObjectRef{Type: 2, Instance: 1}
	if call.obj != wantObj {
		t.Errorf("Write obj = %+v, want %+v", call.obj, wantObj)
	}
	wantReq := session.WriteRequest{Tag: 4, Priority: 8, Value: float32(20)}
	if call.req != wantReq {
		t.Errorf("Write request = %+v, want %+v", call.req, wantReq)
	}
}

func TestLoadDeviceAboveLegacyInstanceSetsExactStatus(t *testing.T) {
	fake := &fakeBrowserSession{}
	view := newBrowserTestView(t, fake)

	statusDone := make(chan struct{})
	go func() {
		view.LoadDevice(store.DeviceRow{Key: store.DeviceKey{Instance: 70000, IP: "192.0.2.1"}})
		close(statusDone)
	}()
	awaitBrowser(t, statusDone)

	deadline := time.Now().Add(2 * time.Second)
	for view.shell.Status.Text == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if got, want := view.shell.Status.Text, "device 70000 needs 22-bit support (pending L2)"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	if len(fake.writeCalls) != 0 {
		t.Errorf("expected no Write calls, got %d", len(fake.writeCalls))
	}
}
