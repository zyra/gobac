// Command gui is the GoBAC Workstation desktop application.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/zyra/gobac/gui/internal/ui"
)

func main() {
	a := app.NewWithID("com.zyra.gobac.gui")

	window := a.NewWindow("GoBAC Workstation")
	window.Resize(fyne.NewSize(1100, 700))
	window.SetMainMenu(ui.NewMainMenu(a, window))
	window.SetContent(ui.NewAppShell(a, window))
	window.ShowAndRun()
}
