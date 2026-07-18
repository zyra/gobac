// Command gui is the GoBAC Workstation desktop application.
package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/zyra/gobac/gui/internal/boot"
	"github.com/zyra/gobac/gui/internal/session"
)

func main() {
	a := app.NewWithID("com.zyra.gobac.gui")
	w := a.NewWindow("GoBAC Workstation")
	boot.Compose(a, w, session.NewLive())
	w.ShowAndRun()
}
