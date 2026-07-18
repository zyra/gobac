// Package boot contains the GoBAC Workstation's single composition root:
// everything that wires a fyne.App and fyne.Window into a running AppShell.
// main() and rendered-launch regression tests both call Compose so the
// exact tree that ships is the exact tree that gets tested.
package boot

import (
	"fmt"

	"fyne.io/fyne/v2"

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
	w.SetMainMenu(ui.NewMainMenu(a, w))

	shell := ui.NewAppShell(a, w)

	settings := ui.LoadSettings(a)
	if err := session.StartFromSettings(sess, settings.Interface, settings.Port); err != nil {
		// Non-fatal: the simulator quickstart and scenario editor don't
		// need a running session, so launch continues and the failure is
		// surfaced in the status bar instead of aborting.
		shell.SetStatus(fmt.Sprintf("Session not started: %v", err))
	}
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
