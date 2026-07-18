package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// homeHeading is the Home view's welcome text (task U4, the UX vision's
// "something, immediately" first screen).
const homeHeading = "Welcome to GoBAC Workstation"

// homeSubtitle is the Home view's one-line, plain-language explanation of
// what the two primary buttons do.
const homeSubtitle = "Explore BACnet devices on your network, or practice on a simulated one."

// NewHomeView builds the Home nav entry: a welcoming heading, a
// plain-language subtitle, two large primary actions — "Simulate a network"
// and "Discover my network" — and a secondary "Settings…" button, so a
// first-time user reaches something useful without ever seeing the word
// "interface". onSimulate, onDiscover, and onSettings are invoked when the
// respective button is tapped; all three must be non-nil.
func NewHomeView(onSimulate, onDiscover, onSettings func()) fyne.CanvasObject {
	heading := canvas.NewText(homeHeading, theme.Color(theme.ColorNameForeground))
	heading.TextStyle = fyne.TextStyle{Bold: true}
	heading.TextSize = theme.TextSize() * 2
	heading.Alignment = fyne.TextAlignCenter

	subtitle := widget.NewLabel(homeSubtitle)
	subtitle.Alignment = fyne.TextAlignCenter
	subtitle.Wrapping = fyne.TextWrapWord

	simulateBtn := widget.NewButtonWithIcon("Simulate a network", theme.ComputerIcon(), onSimulate)
	simulateBtn.Importance = widget.HighImportance

	discoverBtn := widget.NewButtonWithIcon("Discover my network", theme.SearchIcon(), onDiscover)
	discoverBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButtonWithIcon("Settings…", theme.SettingsIcon(), onSettings)

	actions := container.NewGridWithColumns(2, simulateBtn, discoverBtn)

	content := container.NewVBox(
		heading,
		subtitle,
		widget.NewSeparator(),
		actions,
		container.NewCenter(settingsBtn),
	)

	return container.NewCenter(content)
}
