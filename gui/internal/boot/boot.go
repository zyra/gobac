// Package boot contains the GoBAC Workstation's single composition root:
// everything that wires a fyne.App and fyne.Window into a running AppShell.
// main() and rendered-launch regression tests both call Compose so the
// exact tree that ships is the exact tree that gets tested.
package boot

import (
	"fmt"
	"net"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/netpick"
	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/gui/internal/ui"
)

// homeNavIndex, explorerNavIndex, and simulatorNavIndex are the AppShell nav
// indices of the Home, Network Explorer, and Simulator views (see navLabels
// in internal/ui/shell.go). Task U4 added Home and renamed Discovery to
// Network Explorer; the former Object Browser nav row is gone — viewing a
// device's objects is now a drill-down inside the Network Explorer slot
// (see newExplorerPane), not a separate top-level destination.
const (
	homeNavIndex      = 0
	explorerNavIndex  = 1
	simulatorNavIndex = 2
)

// Compose builds the application shell and wires it to sess: window sizing
// and main menu, persisted settings, session start (non-fatal on failure),
// close intercept, stores, every view, Home's primary actions, the Network
// Explorer's device-row-to-Object-Browser drill-down, and window.SetContent.
// It returns the composed shell so callers (and tests) can drive it further.
func Compose(a fyne.App, w fyne.Window, sess session.Session) *ui.AppShell {
	w.Resize(fyne.NewSize(1100, 700))

	shell := ui.NewAppShell(a, w)

	restart := func(s ui.Settings) {
		_ = session.Shutdown(sess)
		startSession(sess, shell, s)
	}
	w.SetMainMenu(ui.NewMainMenu(a, w, restart))

	// Startup uses startLaunch rather than the startSession helper Settings'
	// Restart callback uses: a failed first launch must never greet the user
	// with a raw error (see the UX vision), so it reports gentler wording. A
	// successful launch and a successful restart still read identically.
	startLaunch(sess, shell, ui.LoadSettings(a))

	w.SetCloseIntercept(func() {
		_ = session.Shutdown(sess)
		w.Close()
	})

	devices := store.NewDeviceStore()
	objects := store.NewObjectCache()

	discovery := ui.NewDiscoveryView(sess, devices, shell)
	browser := ui.NewBrowserView(sess, objects, shell)

	explorer := newExplorerPane(discovery, browser)
	shell.SetView(explorerNavIndex, explorer.content)

	editor := ui.NewEditorView(devices, shell)
	shell.SetView(simulatorNavIndex, editor)
	if editorView, ok := editor.(*ui.EditorView); ok {
		editorView.PortHint = func(ports []uint16) string {
			return sessionPortHint(a, ports)
		}
	}

	discoveryView, _ := discovery.(*ui.DiscoveryView)
	browserView, _ := browser.(*ui.BrowserView)
	if discoveryView != nil && browserView != nil {
		discoveryView.OnSelect = func(row store.DeviceRow) {
			browserView.LoadDevice(row)
			explorer.showDetail()
		}
	}

	home := ui.NewHomeView(
		func() { shell.Select(simulatorNavIndex) },
		func() {
			explorer.showList()
			shell.Select(explorerNavIndex)
			if discoveryView != nil {
				discoveryView.Sweep()
			}
		},
		func() { ui.NewSettingsDialog(a, w, restart).Show() },
	)
	shell.SetView(homeNavIndex, home)
	shell.Select(homeNavIndex)

	w.SetContent(shell)

	return shell
}

// explorerPane composes the Network Explorer nav slot (task U4): the device
// list (Discovery) is the default view; selecting a device row swaps in the
// Object Browser detail with a "Back to results" button, so browsing a
// device's objects reads as a drill-down of Network Explorer rather than a
// separate top-level destination — only Home, Network Explorer, and
// Simulator are real nav rows in the rework.
type explorerPane struct {
	content *fyne.Container
	list    fyne.CanvasObject
	detail  *fyne.Container
}

// newExplorerPane wraps list (the Discovery view) and detail (the Browser
// view) into a single content object for shell.SetView(explorerNavIndex, ...).
func newExplorerPane(list, detail fyne.CanvasObject) *explorerPane {
	p := &explorerPane{list: list}

	back := widget.NewButtonWithIcon("Back to results", theme.NavigateBackIcon(), p.showList)
	p.detail = container.NewBorder(back, nil, nil, nil, detail)
	p.detail.Hide()

	p.content = container.NewStack(list, p.detail)
	return p
}

// showDetail swaps the pane to the Object Browser detail (a device row was
// selected in the device list).
func (p *explorerPane) showDetail() {
	p.list.Hide()
	p.detail.Show()
}

// showList swaps the pane back to the device list — via the "Back to
// results" button, or programmatically before a fresh scan (Home's
// "Discover my network" always starts from the list, never mid-drill-down).
func (p *explorerPane) showList() {
	p.detail.Hide()
	p.list.Show()
}

// startLaunch starts sess using s for the app's very first launch and
// reports the outcome in shell's status bar. Success reads identically to a
// Settings restart ("Connected on <label> (port <p>)"); failure uses the
// plain "Not connected yet — check Settings" wording (task U4) rather than
// startSession's raw error text, since a first-run user hasn't done
// anything yet and must never be greeted with an error as the first thing
// on screen (see the UX vision). The detailed error remains available:
// opening Settings and retrying goes through startSession, which does
// report it. A failure is non-fatal either way — the Simulator and Home
// don't need a running session, so Compose keeps going regardless.
func startLaunch(sess session.Session, shell *ui.AppShell, s ui.Settings) {
	label := interfaceLabel(s.Interface)
	if err := session.StartFromSettings(sess, s.Interface, s.Port); err != nil {
		shell.SetStatus("Not connected yet — check Settings")
		return
	}
	shell.SetStatus(fmt.Sprintf("Connected on %s (port %d)", label, s.Port))
}

// startSession starts sess using s and reports the outcome in shell's
// status bar with plain, human-readable wording naming the resolved
// network's label (not a raw interface name). Used by Settings' Restart
// callback, where the user is actively troubleshooting and the detailed
// failure wording is useful. A failure is non-fatal: the simulator and
// scenario editor don't need a running session, so callers keep going with
// just the status bar reflecting what happened.
func startSession(sess session.Session, shell *ui.AppShell, s ui.Settings) {
	label := interfaceLabel(s.Interface)
	if err := session.StartFromSettings(sess, s.Interface, s.Port); err != nil {
		shell.SetStatus(fmt.Sprintf("Couldn't start on %s: %v", label, err))
		return
	}
	shell.SetStatus(fmt.Sprintf("Connected on %s (port %d)", label, s.Port))
}

// sessionPortHint implements EditorView.PortHint: it reports whether any of
// a just-started simulation's ports matches the session's currently
// configured port, and if not, returns a plain-language tip naming the
// first running port so the user knows what to change Settings -> Port to
// in order to reach these simulated devices through the Network Explorer
// browser. Returns "" (no hint) when there is a match or no running
// devices at all.
func sessionPortHint(a fyne.App, ports []uint16) string {
	if len(ports) == 0 {
		return ""
	}
	settings := ui.LoadSettings(a)
	for _, p := range ports {
		if int(p) == settings.Port {
			return ""
		}
	}
	return fmt.Sprintf("Tip: set Settings → Port to %d to interact with these devices.", ports[0])
}

// interfaceLabel returns the human-friendly netpick label for iface (a
// Settings.Interface value): "All networks" for session.AllNetworksSentinel,
// the Automatic pick's label when iface is empty, or the matching
// candidate's label for a named interface. It falls back to "Automatic" or
// the raw name when netpick has nothing to say (e.g. no usable interface,
// or one that has since disappeared) so the status bar always has
// something plain to show.
func interfaceLabel(iface string) string {
	if iface == session.AllNetworksSentinel {
		return "All networks"
	}
	cands := netpick.Candidates(net.Interfaces)
	if iface == "" {
		if c, ok := netpick.Automatic(cands); ok {
			return c.Label
		}
		return "Automatic"
	}
	for _, c := range cands {
		if c.Name == iface {
			return c.Label
		}
	}
	return iface
}
