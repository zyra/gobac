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

// awaitStatus polls shell's rendered status label until it is non-empty or
// the deadline passes, then returns its text. SetStatus dispatches via
// fyne.Do, so a freshly-composed shell's status may not be set yet on the
// calling goroutine.
func awaitStatus(t *testing.T, shell *ui.AppShell) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := shell.Status.Text; s != "" {
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

// TestComposeReportsFailureStatusWhenStartFails exercises the plain
// failure wording when the session fails to start; launch must still
// continue (Compose returns a usable shell) since other views don't depend
// on a running session.
func TestComposeReportsFailureStatusWhenStartFails(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()

	shell := Compose(a, w, failingSession{})

	got := awaitStatus(t, shell)
	if !strings.HasPrefix(got, "Couldn't start on ") || !strings.Contains(got, "bind failed") {
		t.Errorf("status = %q, want prefix %q containing %q", got, "Couldn't start on ", "bind failed")
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
