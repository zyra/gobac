// Command gui is the GoBAC Workstation desktop application.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/gui/internal/ui"
)

// discoveryNavIndex is the AppShell nav index of the Discovery view (see
// navLabels in internal/ui/shell.go).
const discoveryNavIndex = 0

func main() {
	a := app.NewWithID("com.zyra.gobac.gui")

	window := a.NewWindow("GoBAC Workstation")
	window.Resize(fyne.NewSize(1100, 700))
	window.SetMainMenu(ui.NewMainMenu(a, window))

	shell := ui.NewAppShell(a, window)

	sess := session.NewLive()
	devices := store.NewDeviceStore()
	shell.SetView(discoveryNavIndex, ui.NewDiscoveryView(sess, devices, shell))

	window.SetContent(shell)
	window.ShowAndRun()
}
