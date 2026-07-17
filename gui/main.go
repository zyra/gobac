// Command gui is the GoBAC Workstation desktop application.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

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

func main() {
	a := app.NewWithID("com.zyra.gobac.gui")

	window := a.NewWindow("GoBAC Workstation")
	window.Resize(fyne.NewSize(1100, 700))
	window.SetMainMenu(ui.NewMainMenu(a, window))

	shell := ui.NewAppShell(a, window)

	sess := session.NewLive()
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

	window.SetContent(shell)
	window.ShowAndRun()
}
