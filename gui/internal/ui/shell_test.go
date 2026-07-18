package ui

import (
	"image"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestNewAppShellNavigationHasFourLabeledItems(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("test")
	defer w.Close()

	shell := NewAppShell(a, w)

	want := []string{"Discovery", "Object Browser", "Simulator Editor", "Quickstart"}

	if got := len(navLabels); got != len(want) {
		t.Fatalf("len(navLabels) = %d, want %d", got, len(want))
	}
	if shell.Nav == nil {
		t.Fatal("shell.Nav is nil")
	}

	// Render each row through the exact UpdateItem callback the List
	// widget uses, and assert the produced label text.
	for i, wantLabel := range want {
		row := widget.NewLabel("")
		shell.Nav.UpdateItem(widget.ListItemID(i), row)
		if got := row.Text; got != wantLabel {
			t.Errorf("nav row %d text = %q, want %q", i, got, wantLabel)
		}
	}
}

func TestSelectingNavIndexSwitchesVisibleContent(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("test")
	defer w.Close()

	shell := NewAppShell(a, w)
	w.SetContent(shell)
	w.Resize(fyne.NewSize(900, 600))

	shell.Nav.Select(0)
	first := w.Canvas().Capture()

	shell.Nav.Select(2)
	second := w.Canvas().Capture()

	// Rendered assertion: what the driver actually paints must change when
	// the nav selection changes, not just internal container state.
	if imagesEqual(first, second) {
		t.Fatal("canvas capture is unchanged after selecting a different nav row")
	}

	if got, want := visibleLabelText(t, shell.Content), "Scenario editor"; got != want {
		t.Errorf("visible content = %q, want %q", got, want)
	}

	visibleCount := 0
	for _, obj := range shell.Content.Objects {
		if obj.Visible() {
			visibleCount++
		}
	}
	if visibleCount != 1 {
		t.Errorf("visible child count = %d, want 1", visibleCount)
	}
}

func TestSetStatusUpdatesStatusLabel(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("test")
	defer w.Close()

	shell := NewAppShell(a, w)

	done := make(chan struct{})
	go func() {
		shell.SetStatus("ready")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		// Generous for slow CI runners (see awaitBrowser in
		// browser_test.go); a passing wait returns immediately.
		t.Fatal("SetStatus did not return within timeout")
	}

	deadline := time.Now().Add(30 * time.Second)
	for shell.Status.Text != "ready" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if got, want := shell.Status.Text, "ready"; got != want {
		t.Errorf("status label text = %q, want %q", got, want)
	}
}

// visibleLabelText returns the text of the single visible *widget.Label
// child of stack, failing the test if there is not exactly one.
func visibleLabelText(t *testing.T, stack *fyne.Container) string {
	t.Helper()
	var found string
	count := 0
	for _, obj := range stack.Objects {
		if !obj.Visible() {
			continue
		}
		count++
		lbl, ok := obj.(*widget.Label)
		if !ok {
			t.Fatalf("visible object is not a *widget.Label: %T", obj)
		}
		found = lbl.Text
	}
	if count != 1 {
		t.Fatalf("expected exactly one visible child, got %d", count)
	}
	return found
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
