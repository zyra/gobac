package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Preference keys under app.Preferences().
const (
	prefKeyInterface = "iface"
	prefKeyPort      = "port"

	// DefaultPort is the standard BACnet/IP UDP port, used when no port
	// preference has been saved yet.
	DefaultPort = 47808
)

// Settings holds the user-configurable network settings.
type Settings struct {
	Interface string
	Port      int
}

// LoadSettings reads Settings from app.Preferences(), defaulting Port to
// DefaultPort when unset.
func LoadSettings(a fyne.App) Settings {
	prefs := a.Preferences()
	return Settings{
		Interface: prefs.StringWithFallback(prefKeyInterface, ""),
		Port:      prefs.IntWithFallback(prefKeyPort, DefaultPort),
	}
}

// SaveSettings persists Settings to app.Preferences().
func SaveSettings(a fyne.App, s Settings) {
	prefs := a.Preferences()
	prefs.SetString(prefKeyInterface, s.Interface)
	prefs.SetInt(prefKeyPort, s.Port)
}

// validatePort validates a UDP port entry: must be an integer in [1, 65535].
func validatePort(text string) error {
	n, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// trySaveSettings validates portText (via validatePort) and, if valid,
// persists Settings{iface, portText} to a. It returns the validation error
// without saving anything when portText is out of range, so the settings
// dialog's confirm handler can't persist a port validatePort would have
// rejected.
func trySaveSettings(a fyne.App, iface, portText string) error {
	if err := validatePort(portText); err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return err
	}
	SaveSettings(a, Settings{Interface: iface, Port: port})
	return nil
}

// NewSettingsDialog builds the "Settings…" dialog: Interface and UDP Port
// fields, seeded from the current preferences, saving on confirm.
func NewSettingsDialog(a fyne.App, w fyne.Window) dialog.Dialog {
	current := LoadSettings(a)

	ifaceEntry := widget.NewEntry()
	ifaceEntry.SetText(current.Interface)

	portEntry := widget.NewEntry()
	portEntry.SetText(strconv.Itoa(current.Port))
	portEntry.Validator = validatePort

	form := widget.NewForm(
		widget.NewFormItem("Interface", ifaceEntry),
		widget.NewFormItem("UDP Port", portEntry),
	)

	return dialog.NewCustomConfirm("Settings…", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		if err := trySaveSettings(a, ifaceEntry.Text, portEntry.Text); err != nil {
			dialog.ShowError(err, w)
			return
		}
	}, w)
}

// NewMainMenu builds the application main menu, including the "Settings…"
// item that opens the settings dialog.
func NewMainMenu(a fyne.App, w fyne.Window) *fyne.MainMenu {
	settingsItem := fyne.NewMenuItem("Settings…", func() {
		NewSettingsDialog(a, w).Show()
	})
	fileMenu := fyne.NewMenu("File", settingsItem)
	return fyne.NewMainMenu(fileMenu)
}
