package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
)

func TestValidatePortRejectsInvalidValues(t *testing.T) {
	cases := []string{"0", "70000", "abc"}
	for _, in := range cases {
		if err := validatePort(in); err == nil {
			t.Errorf("validatePort(%q) = nil, want error", in)
		}
	}
}

func TestValidatePortAcceptsInRangeValue(t *testing.T) {
	if err := validatePort("47808"); err != nil {
		t.Errorf("validatePort(%q) = %v, want nil", "47808", err)
	}
}

func TestLoadSettingsDefaultsPortWhenUnset(t *testing.T) {
	a := test.NewApp()

	got := LoadSettings(a)

	if got.Port != DefaultPort {
		t.Errorf("LoadSettings(a).Port = %d, want %d", got.Port, DefaultPort)
	}
	if got.Interface != "" {
		t.Errorf("LoadSettings(a).Interface = %q, want empty", got.Interface)
	}
}

func TestTrySaveSettingsRejectsOutOfRangePort(t *testing.T) {
	a := test.NewApp()
	SaveSettings(a, Settings{Interface: "eno0", Port: DefaultPort})

	if err := trySaveSettings(a, "eth9", "70000"); err == nil {
		t.Fatal("trySaveSettings with out-of-range port = nil error, want error")
	}

	got := LoadSettings(a)
	if got.Interface != "eno0" || got.Port != DefaultPort {
		t.Errorf("LoadSettings(a) = %+v, want unchanged Interface %q Port %d", got, "eno0", DefaultPort)
	}
}

func TestTrySaveSettingsPersistsValidPort(t *testing.T) {
	a := test.NewApp()

	if err := trySaveSettings(a, "eno0", "47809"); err != nil {
		t.Fatalf("trySaveSettings: %v", err)
	}

	got := LoadSettings(a)
	if got.Interface != "eno0" || got.Port != 47809 {
		t.Errorf("LoadSettings(a) = %+v, want Interface %q Port %d", got, "eno0", 47809)
	}
}

// TestNewSettingsDialogRendersAllNetworksThenAutomaticFirst renders the real
// dialog (not just its constructor return value) under the Fyne test driver
// and reads the actual rendered widget.Select: its first two options must be
// "All networks" then "Automatic (recommended)" (task U5 adds "All
// networks" ahead of the existing Automatic entry), and with no interface
// saved yet Automatic must be the pre-selected value.
func TestNewSettingsDialogRendersAllNetworksThenAutomaticFirst(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))

	NewSettingsDialog(a, w, nil).Show()

	sel := findSelect(w.Canvas().Overlays().Top())
	if sel == nil {
		t.Fatal("no widget.Select found in rendered settings dialog")
	}
	if len(sel.Options) < 2 || sel.Options[0] != allNetworksLabel || sel.Options[1] != automaticLabel {
		t.Fatalf("Select.Options = %v, want first two entries %q, %q", sel.Options, allNetworksLabel, automaticLabel)
	}
	if sel.Selected != automaticLabel {
		t.Errorf("Select.Selected = %q, want %q (no interface saved yet)", sel.Selected, automaticLabel)
	}
}

// TestNewSettingsDialogSelectingAllNetworksPersistsSentinelAndRestarts
// drives the rendered Select to "All networks" and taps the real "Save"
// button, asserting the outcome: the persisted Settings.Interface is
// session.AllNetworksSentinel (never the display label), and the restart
// callback receives the same value.
func TestNewSettingsDialogSelectingAllNetworksPersistsSentinelAndRestarts(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))

	var restarted Settings
	restartCalls := 0
	NewSettingsDialog(a, w, func(s Settings) {
		restartCalls++
		restarted = s
	}).Show()

	top := w.Canvas().Overlays().Top()

	sel := findSelect(top)
	if sel == nil {
		t.Fatal("no widget.Select found in rendered settings dialog")
	}
	sel.SetSelected(allNetworksLabel)

	save := findButton(top, "Save")
	if save == nil {
		t.Fatal("no \"Save\" button found in rendered settings dialog")
	}
	test.Tap(save)

	if restartCalls != 1 {
		t.Fatalf("restart called %d times, want 1", restartCalls)
	}
	if restarted.Interface != session.AllNetworksSentinel {
		t.Errorf("restart Settings.Interface = %q, want %q", restarted.Interface, session.AllNetworksSentinel)
	}

	got := LoadSettings(a)
	if got.Interface != session.AllNetworksSentinel {
		t.Errorf("LoadSettings(a).Interface = %q, want %q", got.Interface, session.AllNetworksSentinel)
	}
}

// TestLabelForInterfaceRoundTripsAllNetworksSentinel is a plain regression
// check (no rendering needed -- labelForInterface has no Fyne dependency)
// that persisting session.AllNetworksSentinel and reloading it displays the
// "All networks" label rather than falling back to Automatic.
func TestLabelForInterfaceRoundTripsAllNetworksSentinel(t *testing.T) {
	got := labelForInterface(session.AllNetworksSentinel)
	if got != allNetworksLabel {
		t.Errorf("labelForInterface(%q) = %q, want %q", session.AllNetworksSentinel, got, allNetworksLabel)
	}
}

// TestNewSettingsDialogSavesSelectedInterfaceAndRestarts drives the
// rendered Select and Port entry, taps the real "Save" button found in the
// dialog's rendered object tree, and asserts on the outcome: the injected
// restart callback fires with the persisted Settings, and prefs hold the
// chosen interface's name (not its label, which embeds a volatile IP).
func TestNewSettingsDialogSavesSelectedInterfaceAndRestarts(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))

	var restarted Settings
	restartCalls := 0
	NewSettingsDialog(a, w, func(s Settings) {
		restartCalls++
		restarted = s
	}).Show()

	top := w.Canvas().Overlays().Top()

	sel := findSelect(top)
	if sel == nil {
		t.Fatal("no widget.Select found in rendered settings dialog")
	}
	if len(sel.Options) < 2 {
		t.Fatal("expected at least one real candidate besides Automatic; the host's loopback interface should always qualify")
	}
	// The candidate list sorts non-loopback first, loopback last, so the
	// last option is always present and deterministic across hosts.
	chosen := sel.Options[len(sel.Options)-1]
	sel.SetSelected(chosen)

	entry := findEntry(top)
	if entry == nil {
		t.Fatal("no widget.Entry found in rendered settings dialog")
	}
	entry.SetText("47809")

	save := findButton(top, "Save")
	if save == nil {
		t.Fatal("no \"Save\" button found in rendered settings dialog")
	}
	test.Tap(save)

	if restartCalls != 1 {
		t.Fatalf("restart called %d times, want 1", restartCalls)
	}
	if restarted.Port != 47809 {
		t.Errorf("restart Settings.Port = %d, want 47809", restarted.Port)
	}
	if restarted.Interface == "" {
		t.Errorf("restart Settings.Interface = empty, want the chosen candidate's interface name")
	}

	got := LoadSettings(a)
	if got.Port != 47809 || got.Interface != restarted.Interface {
		t.Errorf("LoadSettings(a) = %+v, want Port 47809 Interface %q", got, restarted.Interface)
	}
}

// TestNewSettingsDialogInvalidPortShowsErrorAndSkipsSaveAndRestart taps the
// rendered "Save" button with an out-of-range port and asserts the outcome
// on rendered state: a new overlay (the error dialog) appears on the
// canvas, nothing was persisted, and restart never fired.
func TestNewSettingsDialogInvalidPortShowsErrorAndSkipsSaveAndRestart(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("t")
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))

	SaveSettings(a, Settings{Interface: "", Port: DefaultPort})

	restartCalls := 0
	NewSettingsDialog(a, w, func(Settings) { restartCalls++ }).Show()

	top := w.Canvas().Overlays().Top()

	entry := findEntry(top)
	if entry == nil {
		t.Fatal("no widget.Entry found in rendered settings dialog")
	}
	entry.SetText("70000")

	save := findButton(top, "Save")
	if save == nil {
		t.Fatal("no \"Save\" button found in rendered settings dialog")
	}
	test.Tap(save)

	// A successful Save hides the settings dialog's own popup before
	// calling back; an invalid port instead pushes a new error dialog over
	// it, so the canvas's top overlay must have changed to something new.
	newTop := w.Canvas().Overlays().Top()
	if newTop == nil || newTop == top {
		t.Fatal("no new overlay appeared after an invalid-port Save; want an error dialog rendered on top")
	}

	if restartCalls != 0 {
		t.Errorf("restart called %d times after invalid port, want 0", restartCalls)
	}
	got := LoadSettings(a)
	if got.Port != DefaultPort {
		t.Errorf("LoadSettings(a).Port = %d, want unchanged %d", got.Port, DefaultPort)
	}
}

// walkCanvasObject recurses into obj -- through *fyne.Container.Objects and,
// for any fyne.Widget, its rendered test.WidgetRenderer(obj).Objects() --
// calling visit on every object reached (including obj itself) until visit
// reports a match.
func walkCanvasObject(obj fyne.CanvasObject, visit func(fyne.CanvasObject) bool) bool {
	if obj == nil {
		return false
	}
	if visit(obj) {
		return true
	}
	if c, ok := obj.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if walkCanvasObject(child, visit) {
				return true
			}
		}
		return false
	}
	if wid, ok := obj.(fyne.Widget); ok {
		r := test.WidgetRenderer(wid)
		if r == nil {
			return false
		}
		for _, child := range r.Objects() {
			if walkCanvasObject(child, visit) {
				return true
			}
		}
	}
	return false
}

// findSelect returns the first *widget.Select reachable from root.
func findSelect(root fyne.CanvasObject) *widget.Select {
	var found *widget.Select
	walkCanvasObject(root, func(o fyne.CanvasObject) bool {
		s, ok := o.(*widget.Select)
		if ok {
			found = s
		}
		return ok
	})
	return found
}

// findEntry returns the first *widget.Entry reachable from root.
func findEntry(root fyne.CanvasObject) *widget.Entry {
	var found *widget.Entry
	walkCanvasObject(root, func(o fyne.CanvasObject) bool {
		e, ok := o.(*widget.Entry)
		if ok {
			found = e
		}
		return ok
	})
	return found
}

// findButton returns the first *widget.Button reachable from root whose
// Text matches text.
func findButton(root fyne.CanvasObject, text string) *widget.Button {
	var found *widget.Button
	walkCanvasObject(root, func(o fyne.CanvasObject) bool {
		b, ok := o.(*widget.Button)
		if ok && b.Text == text {
			found = b
			return true
		}
		return false
	})
	return found
}
