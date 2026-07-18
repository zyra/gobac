// Package boot contains the GoBAC Workstation's single composition root:
// everything that wires a fyne.App and fyne.Window into a running AppShell.
// main() and rendered-launch regression tests both call Compose so the
// exact tree that ships is the exact tree that gets tested.
package boot

import (
	"fmt"
	"net"

	"fyne.io/fyne/v2"

	"github.com/zyra/gobac/gui/internal/netpick"
	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/gui/internal/ui"
)

// discoveryNavIndex, browserNavIndex, editorNavIndex, and quickstartNavIndex
// are the AppShell nav indices of the Discovery, Object Browser, Simulator
// Editor, and Quickstart views (see navLabels in internal/ui/shell.go).
const (
	discoveryNavIndex  = 0
	browserNavIndex    = 1
	editorNavIndex     = 2
	quickstartNavIndex = 3
)

// Compose builds the application shell and wires it to sess: window sizing
// and main menu, persisted settings, session start (non-fatal on failure),
// close intercept, stores, every view, the Discovery-to-Browser selection
// handoff, and window.SetContent. It returns the composed shell so callers
// (and tests) can drive it further.
func Compose(a fyne.App, w fyne.Window, sess session.Session) *ui.AppShell {
	w.Resize(fyne.NewSize(1100, 700))

	shell := ui.NewAppShell(a, w)

	restart := func(s ui.Settings) {
		_ = session.Shutdown(sess)
		startSession(sess, shell, s)
	}
	w.SetMainMenu(ui.NewMainMenu(a, w, restart))

	// Startup goes through the same startSession helper Settings' Restart
	// callback uses, so first launch and a live network change report
	// identically in the status bar.
	startSession(sess, shell, ui.LoadSettings(a))

	w.SetCloseIntercept(func() {
		_ = session.Shutdown(sess)
		w.Close()
	})

	devices := store.NewDeviceStore()
	objects := store.NewObjectCache()

	discovery := ui.NewDiscoveryView(sess, devices, shell)
	shell.SetView(discoveryNavIndex, discovery)

	browser := ui.NewBrowserView(sess, objects, shell)
	shell.SetView(browserNavIndex, browser)

	editor := ui.NewEditorView(shell)
	shell.SetView(editorNavIndex, editor)

	quickstart := ui.NewQuickstartView(devices, shell)
	shell.SetView(quickstartNavIndex, quickstart)

	if discoveryView, ok := discovery.(*ui.DiscoveryView); ok {
		if browserView, ok := browser.(*ui.BrowserView); ok {
			discoveryView.OnSelect = func(row store.DeviceRow) {
				browserView.LoadDevice(row)
				shell.Nav.Select(browserNavIndex)
			}
		}
	}

	w.SetContent(shell)

	return shell
}

// startSession starts sess using s and reports the outcome in shell's
// status bar with plain, human-readable wording naming the resolved
// network's label (not a raw interface name). A failure is non-fatal: the
// simulator quickstart and scenario editor don't need a running session,
// so callers keep going with just the status bar reflecting what happened.
func startSession(sess session.Session, shell *ui.AppShell, s ui.Settings) {
	label := interfaceLabel(s.Interface)
	if err := session.StartFromSettings(sess, s.Interface, s.Port); err != nil {
		shell.SetStatus(fmt.Sprintf("Couldn't start on %s: %v", label, err))
		return
	}
	shell.SetStatus(fmt.Sprintf("Connected on %s (port %d)", label, s.Port))
}

// interfaceLabel returns the human-friendly netpick label for iface (a
// Settings.Interface value): the Automatic pick's label when iface is
// empty, or the matching candidate's label for a named interface. It falls
// back to "Automatic" or the raw name when netpick has nothing to say
// (e.g. no usable interface, or one that has since disappeared) so the
// status bar always has something plain to show.
func interfaceLabel(iface string) string {
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
