package ui

import (
	"fmt"
	"net"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/netpick"
)

// automaticLabel is the network Select's first option: choosing it persists
// an empty Settings.Interface, which StartFromSettings resolves to the best
// real interface at start time.
const automaticLabel = "Automatic (recommended)"

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

// networkOptions returns the network Select's option strings (Automatic
// first, then each netpick candidate's label) plus a lookup from label back
// to the interface name that should be persisted ("" for Automatic).
func networkOptions() (options []string, nameByLabel map[string]string) {
	cands := netpick.Candidates(net.Interfaces)

	options = make([]string, 0, len(cands)+1)
	options = append(options, automaticLabel)
	nameByLabel = map[string]string{automaticLabel: ""}

	for _, c := range cands {
		options = append(options, c.Label)
		nameByLabel[c.Label] = c.Name
	}

	return options, nameByLabel
}

// labelForInterface returns the Select label that corresponds to iface
// (an interface name as persisted in Settings), falling back to
// automaticLabel when iface is empty or unrecognized (e.g. an interface
// that has since disappeared).
func labelForInterface(iface string) string {
	if iface == "" {
		return automaticLabel
	}
	cands := netpick.Candidates(net.Interfaces)
	for _, c := range cands {
		if c.Name == iface {
			return c.Label
		}
	}
	return automaticLabel
}

// NewSettingsDialog builds the "Settings…" dialog: Network and Port fields,
// seeded from the current preferences, saving on confirm. restart, if
// non-nil, is called with the newly persisted Settings after a successful
// save so the caller can restart the session against the new network
// choice; it is not called when validation fails or the user cancels.
func NewSettingsDialog(a fyne.App, w fyne.Window, restart func(Settings)) dialog.Dialog {
	current := LoadSettings(a)

	options, nameByLabel := networkOptions()

	networkSelect := widget.NewSelect(options, nil)
	networkSelect.SetSelected(labelForInterface(current.Interface))

	portEntry := widget.NewEntry()
	portEntry.SetText(strconv.Itoa(current.Port))
	portEntry.Validator = validatePort

	form := widget.NewForm(
		widget.NewFormItem("Network", networkSelect),
		widget.NewFormItem("Port", portEntry),
	)

	return dialog.NewCustomConfirm("Settings…", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		iface := nameByLabel[networkSelect.Selected]
		if err := trySaveSettings(a, iface, portEntry.Text); err != nil {
			dialog.ShowError(err, w)
			return
		}
		if restart != nil {
			restart(LoadSettings(a))
		}
	}, w)
}

// NewMainMenu builds the application main menu, including the "Settings…"
// item that opens the settings dialog. restart is forwarded to
// NewSettingsDialog (see its doc comment).
func NewMainMenu(a fyne.App, w fyne.Window, restart func(Settings)) *fyne.MainMenu {
	settingsItem := fyne.NewMenuItem("Settings…", func() {
		NewSettingsDialog(a, w, restart).Show()
	})
	fileMenu := fyne.NewMenu("File", settingsItem)
	return fyne.NewMainMenu(fileMenu)
}
