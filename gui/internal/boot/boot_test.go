package boot

import (
	"context"
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/ui"
)

// fakeSession is a minimal session.Session whose Start succeeds without
// touching any socket, so Compose can be exercised under the Fyne test
// driver without binding real network resources.
type fakeSession struct{}

var _ session.Session = fakeSession{}

func (fakeSession) Start(session.Config) error { return nil }
func (fakeSession) Stop() error                { return nil }

func (fakeSession) Discover(ctx context.Context, timeout time.Duration) (<-chan session.DeviceSummary, error) {
	ch := make(chan session.DeviceSummary)
	close(ch)
	return ch, nil
}

func (fakeSession) ReadProperty(ctx context.Context, dev session.Address, obj session.ObjectRef, prop uint32) ([]session.Value, error) {
	return nil, nil
}

func (fakeSession) ReadMultiple(ctx context.Context, dev session.Address, specs []session.ReadSpec) ([]session.ObjectResult, error) {
	return nil, nil
}

func (fakeSession) Write(ctx context.Context, dev session.Address, obj session.ObjectRef, w session.WriteRequest) error {
	return nil
}

// failingSession is a session.Session whose Start always fails, so
// Compose's failure-path status wording can be exercised without touching
// any socket.
type failingSession struct{ fakeSession }

func (failingSession) Start(session.Config) error { return errors.New("bind failed") }

// awaitStatus polls shell's rendered status label until it differs from
// both "" and the shell's "Ready" launch default, or the deadline passes,
// then returns its text. SetStatus dispatches via fyne.Do, so a
// freshly-composed shell's status may not reflect startLaunch's outcome yet
// on the calling goroutine — it may still read the constructor's "Ready"
// placeholder.
func awaitStatus(t *testing.T, shell *ui.AppShell) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := shell.Status.Text; s != "" && s != "Ready" {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("status bar text was never set")
	return ""
}

// TestComposeReportsConnectedStatusOnSuccessfulStart exercises the plain
// success wording Compose's startup path (and, by the same helper, a
// Settings restart) reports in the rendered status bar.
func TestComposeReportsConnectedStatusOnSuccessfulStart(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, fakeSession{})

	got := awaitStatus(t, shell)
	if !strings.HasPrefix(got, "Connected on ") || !strings.Contains(got, "(port ") {
		t.Errorf("status = %q, want prefix %q and a port", got, "Connected on ")
	}
}

// TestComposeReportsFailureStatusWhenStartFails exercises the plain,
// first-run-safe failure wording (task U4) when the session fails to start
// at launch — never a raw error as the first thing on screen — and that
// launch still continues (Compose returns a usable shell, Home still
// renders) since other views don't depend on a running session.
func TestComposeReportsFailureStatusWhenStartFails(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, failingSession{})

	got := awaitStatus(t, shell)
	want := "Not connected yet — check Settings"
	if got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

// TestSessionPortHintMatchesRunningPortReturnsEmpty covers sessionPortHint's
// no-hint case: the session's configured port already matches one of the
// simulation's running ports, so no Settings tip is needed.
func TestSessionPortHintMatchesRunningPortReturnsEmpty(t *testing.T) {
	a := test.NewApp()
	ui.SaveSettings(a, ui.Settings{Interface: "eno0", Port: 47902})

	got := sessionPortHint(a, []uint16{47901, 47902})
	if got != "" {
		t.Errorf("sessionPortHint = %q, want \"\" (session port matches a running port)", got)
	}
}

// TestSessionPortHintMismatchReturnsTipNamingFirstPort covers the actual
// tip: no running port matches the session's configured port, so the hint
// names the first running port.
func TestSessionPortHintMismatchReturnsTipNamingFirstPort(t *testing.T) {
	a := test.NewApp()
	ui.SaveSettings(a, ui.Settings{Interface: "eno0", Port: 47808})

	got := sessionPortHint(a, []uint16{47901, 47902})
	want := "Tip: set Settings → Port to 47901 to interact with these devices."
	if got != want {
		t.Errorf("sessionPortHint = %q, want %q", got, want)
	}
}

// TestSessionPortHintNoRunningDevicesReturnsEmpty covers the degenerate
// zero-ports case (should never happen in practice — Run always injects at
// least the devices it started — but must not panic or fabricate a tip).
func TestSessionPortHintNoRunningDevicesReturnsEmpty(t *testing.T) {
	a := test.NewApp()
	if got := sessionPortHint(a, nil); got != "" {
		t.Errorf("sessionPortHint(nil) = %q, want \"\"", got)
	}
}

// TestComposedWindowRendersNonBlank exercises the exact composition main()
// performs (via Compose) under the Fyne test driver and asserts on the
// rendered pixels, not the container tree. Before AppShell became a proper
// widget, window.SetContent(shell) drew nothing: the driver's software
// renderer didn't recognize the embedded-*fyne.Container promotion, so a
// captured canvas was a solid blank image (1-2 distinct colors). This test
// fails immediately if CreateRenderer is removed or SetContent stops being
// handed a real widget.
func TestComposedWindowRendersNonBlank(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	Compose(a, w, fakeSession{})
	w.Resize(fyne.NewSize(1100, 700))

	img := w.Canvas().Capture()

	colors := distinctColors(img)
	if colors <= 50 {
		t.Fatalf("captured canvas has %d distinct colors, want > 50 (a blank canvas has 1-2)", colors)
	}

	// The content region (right of the nav list, above the status bar)
	// must not be a single flat color either -- guards against a renderer
	// that only paints the nav/status chrome while the center stack stays
	// blank.
	contentColors := distinctColorsInRegion(img, image.Rect(300, 0, 1100, 650))
	if contentColors <= 1 {
		t.Fatalf("captured content region has %d distinct colors, want > 1", contentColors)
	}
}

// TestComposedWindowShowsNavAndStatus drives the real, composed canvas:
// selecting a different nav row must change what's painted, and the
// shell's renderer must actually contain the nav list widget somewhere in
// its object tree (not just as an internal struct field).
func TestComposedWindowShowsNavAndStatus(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, fakeSession{})
	w.Resize(fyne.NewSize(1100, 700))

	shell.Nav.Select(0)
	first := w.Canvas().Capture()

	shell.Nav.Select(2)
	second := w.Canvas().Capture()

	if imagesEqual(first, second) {
		t.Fatal("canvas capture is unchanged after selecting a different nav row")
	}

	renderer := test.WidgetRenderer(shell)
	if renderer == nil {
		t.Fatal("test.WidgetRenderer(shell) returned a nil renderer")
	}
	found := false
	for _, obj := range renderer.Objects() {
		if objectTreeContains(obj, shell.Nav) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("shell's rendered object tree does not include the nav list")
	}
}

// findButton returns the first *widget.Button in obj's tree whose Text
// matches label, failing the test if none is found.
func findButton(t *testing.T, obj fyne.CanvasObject, label string) *widget.Button {
	t.Helper()
	var found *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if btn, ok := o.(*widget.Button); ok && btn.Text == label {
			found = btn
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, child := range c.Objects {
				walk(child)
			}
		}
	}
	walk(obj)
	if found == nil {
		t.Fatalf("no *widget.Button with text %q found in rendered tree", label)
	}
	return found
}

// TestComposedWindowLaunchesWithHomeSelected covers task U4: the app must
// launch on the Home view, and Home must actually be the *selected* nav
// row (not merely the default content shown), so re-selecting it is a
// no-op that changes nothing on screen, while selecting either other row
// changes what's painted.
func TestComposedWindowLaunchesWithHomeSelected(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, fakeSession{})
	w.Resize(fyne.NewSize(1100, 700))

	launch := w.Canvas().Capture()

	shell.Select(homeNavIndex)
	afterReselect := w.Canvas().Capture()
	if !imagesEqual(launch, afterReselect) {
		t.Fatal("re-selecting Home changed the rendered canvas: Home was not already the selected nav row at launch")
	}

	shell.Select(explorerNavIndex)
	if imagesEqual(launch, w.Canvas().Capture()) {
		t.Fatal("launch capture is identical to the Network Explorer capture")
	}

	shell.Select(homeNavIndex)
	shell.Select(simulatorNavIndex)
	if imagesEqual(launch, w.Canvas().Capture()) {
		t.Fatal("launch capture is identical to the Simulator capture")
	}
}

// TestHomeSimulateButtonNavigatesToSimulator covers task U4: tapping
// Home's "Simulate a network" button must render exactly what selecting
// the Simulator nav row directly renders.
func TestHomeSimulateButtonNavigatesToSimulator(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, fakeSession{})
	w.Resize(fyne.NewSize(1100, 700))

	homeContent := shell.Content.Objects[homeNavIndex]
	test.Tap(findButton(t, homeContent, "Simulate a network"))
	afterTap := w.Canvas().Capture()

	shell.Select(homeNavIndex)
	shell.Select(simulatorNavIndex)
	viaNav := w.Canvas().Capture()

	if !imagesEqual(afterTap, viaNav) {
		t.Fatal("tapping Simulate a network did not render the same as selecting the Simulator nav row directly")
	}
}

// discoverySpySession wraps fakeSession, overriding Discover to close
// called the moment it is invoked. Discover runs on the Discovery view's
// own background sweep goroutine (see DiscoveryView.sweep), so a plain poll
// of shared state from the test goroutine would race with it; a channel
// close/receive is a genuine synchronization point regardless of which
// goroutine performs the close, so waiting on it is race-free evidence the
// seam fired without needing to wait for the goroutine's later, unrelated
// work (re-enabling the sweep button, setting the final status).
type discoverySpySession struct {
	fakeSession
	called chan struct{}
}

func (s *discoverySpySession) Discover(ctx context.Context, timeout time.Duration) (<-chan session.DeviceSummary, error) {
	close(s.called)
	ch := make(chan session.DeviceSummary)
	close(ch)
	return ch, nil
}

// TestHomeDiscoverButtonNavigatesToExplorerAndTriggersSweep covers task U4:
// tapping Home's "Discover my network" button must switch the visible
// content to the Network Explorer nav row and must actually trigger a scan.
// Home's wiring selects Network Explorer before calling Sweep, and that
// selection is entirely synchronous, so asserting on Show()/Hide() state
// immediately after the tap is safe — nothing concurrent touches it.
func TestHomeDiscoverButtonNavigatesToExplorerAndTriggersSweep(t *testing.T) {
	fake := &discoverySpySession{called: make(chan struct{})}

	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, fake)
	w.Resize(fyne.NewSize(1100, 700))

	homeContent := shell.Content.Objects[homeNavIndex]
	test.Tap(findButton(t, homeContent, "Discover my network"))

	select {
	case <-fake.called:
	case <-time.After(5 * time.Second):
		t.Fatal("Discover was never called — the sweep seam did not fire")
	}

	if !shell.Content.Objects[explorerNavIndex].Visible() {
		t.Error("Network Explorer content should be visible after tapping Discover my network")
	}
	if shell.Content.Objects[homeNavIndex].Visible() {
		t.Error("Home content should be hidden after tapping Discover my network")
	}
}

// distinctColors returns the number of distinct pixel colors in img.
func distinctColors(img image.Image) int {
	return distinctColorsInRegion(img, img.Bounds())
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

// imagesEqual reports whether a and b have identical bounds and pixels.
func imagesEqual(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

// objectTreeContains reports whether target is root itself or is reachable
// by recursing into root's children when root is a *fyne.Container.
func objectTreeContains(root, target fyne.CanvasObject) bool {
	if root == target {
		return true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if objectTreeContains(child, target) {
				return true
			}
		}
	}
	return false
}
