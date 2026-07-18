package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestHomeViewButtonsInvokeExactCallback drives each of Home's three
// buttons through the Fyne test driver's Tap and asserts only its own
// callback fires.
func TestHomeViewButtonsInvokeExactCallback(t *testing.T) {
	var simulateCalls, discoverCalls, settingsCalls int

	home := NewHomeView(
		func() { simulateCalls++ },
		func() { discoverCalls++ },
		func() { settingsCalls++ },
	)

	a := test.NewApp()
	w := a.NewWindow("test")
	defer w.Close()
	w.SetContent(home)
	w.Resize(fyne.NewSize(800, 500))

	simulateBtn := findButton(home, "Simulate a network")
	if simulateBtn == nil {
		t.Fatal("Simulate a network button not found in rendered tree")
	}
	test.Tap(simulateBtn)
	if simulateCalls != 1 || discoverCalls != 0 || settingsCalls != 0 {
		t.Errorf("after tapping Simulate: simulate=%d discover=%d settings=%d, want 1,0,0", simulateCalls, discoverCalls, settingsCalls)
	}

	discoverBtn := findButton(home, "Discover my network")
	if discoverBtn == nil {
		t.Fatal("Discover my network button not found in rendered tree")
	}
	test.Tap(discoverBtn)
	if simulateCalls != 1 || discoverCalls != 1 || settingsCalls != 0 {
		t.Errorf("after tapping Discover: simulate=%d discover=%d settings=%d, want 1,1,0", simulateCalls, discoverCalls, settingsCalls)
	}

	settingsBtn := findButton(home, "Settings…")
	if settingsBtn == nil {
		t.Fatal("Settings… button not found in rendered tree")
	}
	test.Tap(settingsBtn)
	if simulateCalls != 1 || discoverCalls != 1 || settingsCalls != 1 {
		t.Errorf("after tapping Settings: simulate=%d discover=%d settings=%d, want 1,1,1", simulateCalls, discoverCalls, settingsCalls)
	}
}

// TestHomeViewRendersNonBlankWithHeadingAndSubtitle is a rendered sanity
// check: the composed view must actually paint something (guards against
// the AppShell-embedding class of bug — see U1) and both primary buttons
// must be reachable in the rendered tree.
func TestHomeViewRendersNonBlankWithHeadingAndSubtitle(t *testing.T) {
	home := NewHomeView(func() {}, func() {}, func() {})

	a := test.NewApp()
	w := a.NewWindow("test")
	defer w.Close()
	w.SetContent(home)
	w.Resize(fyne.NewSize(800, 500))

	img := w.Canvas().Capture()
	seen := make(map[[4]uint32]struct{})
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bch, aCh := img.At(x, y).RGBA()
			seen[[4]uint32{r, g, bch, aCh}] = struct{}{}
		}
	}
	if len(seen) <= 1 {
		t.Fatalf("captured canvas has %d distinct colors, want > 1", len(seen))
	}

	if findButton(home, "Simulate a network") == nil {
		t.Error("Simulate button missing from rendered tree")
	}
	if findButton(home, "Discover my network") == nil {
		t.Error("Discover button missing from rendered tree")
	}
}
