// Package ui contains the Fyne views and application shell for the GoBAC
// Workstation. It is the only package in this module that imports Fyne.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// navLabels are the left-navigation entries, in display order. Task U4 adds
// a welcoming Home entry and renames Discovery to the plain-language
// "Network Explorer" (see the UX vision in the gui-ux-rework plan); Object
// Browser is no longer a top-level destination — viewing a device's objects
// is a drill-down of Network Explorer, wired in boot.Compose.
var navLabels = []string{"Home", "Network Explorer", "Simulator"}

// viewLabels are the placeholder center-content texts, one per navLabels
// entry at the same index.
var viewLabels = []string{"Home view", "Network Explorer view", "Simulator"}

// AppShell is the top-level content for the GoBAC Workstation main window:
// a left navigation list, a center content stack that switches per
// selection, and a bottom status bar.
//
// AppShell is a proper widget (widget.BaseWidget + CreateRenderer) rather
// than an embedded *fyne.Container. Embedding *fyne.Container only
// satisfies fyne.CanvasObject by method promotion on the concrete type
// *AppShell; the driver's software renderer recognizes *fyne.Container and
// fyne.Widget by concrete type/interface, not by promotion, so a bare
// embed renders nothing when handed to window.SetContent. Being a widget
// with CreateRenderer makes that class of bug impossible here.
type AppShell struct {
	widget.BaseWidget

	Nav     *widget.List
	Content *fyne.Container
	Status  *widget.Label

	root     *fyne.Container
	selected int
}

// NewAppShell builds the application shell.
func NewAppShell(a fyne.App, w fyne.Window) *AppShell {
	shell := &AppShell{
		Status: widget.NewLabel("Ready"),
	}

	views := make([]fyne.CanvasObject, len(viewLabels))
	for i, text := range viewLabels {
		views[i] = widget.NewLabel(text)
	}
	shell.Content = container.NewStack(views...)
	shell.selectView(0)

	// The List widget creates one item via CreateItem to measure and cache
	// its per-row minimum size (itemMin), and never recomputes it
	// afterward -- see fyne.io/fyne/v2/widget.List.Resize/updateItem. An
	// empty-string template therefore locks the whole list (and the
	// bordered nav rail built from it) to a near-zero width for its
	// entire life, regardless of the real text updateNavItem later fills
	// in: rows still render their real label text, but squeezed into that
	// tiny cached width, so "Network Explorer" paints as "Net". Seeding
	// the template with the widest actual nav label makes the cached
	// itemMin -- and so the rail's natural width -- match the widest row
	// up front.
	shell.Nav = widget.NewList(
		func() int { return len(navLabels) },
		func() fyne.CanvasObject { return widget.NewLabel(widestNavLabel()) },
		updateNavItem,
	)
	shell.Nav.OnSelected = func(id widget.ListItemID) {
		shell.selectView(id)
	}

	statusBar := container.NewHBox(shell.Status)

	shell.root = container.NewBorder(nil, statusBar, shell.Nav, nil, shell.Content)
	shell.ExtendBaseWidget(shell)

	return shell
}

// CreateRenderer implements fyne.Widget.
func (s *AppShell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.root)
}

// updateNavItem renders the navigation row at id into obj, an item created
// by the List's CreateItem callback.
func updateNavItem(id widget.ListItemID, obj fyne.CanvasObject) {
	obj.(*widget.Label).SetText(navLabels[id])
}

// widestNavLabel returns the entry in navLabels that a *widget.Label
// measures widest, by rendered MinSize rather than character count, so it
// stays correct regardless of which characters the widest string actually
// contains.
func widestNavLabel() string {
	widest := navLabels[0]
	widestW := widget.NewLabel(widest).MinSize().Width
	for _, l := range navLabels[1:] {
		if w := widget.NewLabel(l).MinSize().Width; w > widestW {
			widest = l
			widestW = w
		}
	}
	return widest
}

// selectView shows the content view at index id and hides all others.
func (s *AppShell) selectView(id int) {
	s.selected = id
	for i, obj := range s.Content.Objects {
		if i == id {
			obj.Show()
		} else {
			obj.Hide()
		}
	}
}

// SetView replaces the content view at index id with obj, preserving
// whichever view is currently selected (obj is shown if id is the
// currently selected nav index, hidden otherwise). Used to swap a
// placeholder view for a real one after construction.
func (s *AppShell) SetView(id int, obj fyne.CanvasObject) {
	if id == s.selected {
		obj.Show()
	} else {
		obj.Hide()
	}
	s.Content.Objects[id] = obj
	s.Content.Refresh()
}

// Select switches the visible content to the nav row at id, exactly as if
// the user had clicked that row (it fires Nav.OnSelected, which drives
// selectView). Exported so callers outside this package — namely Home's
// primary action buttons, wired in boot.Compose — can navigate without
// reaching into Nav directly.
func (s *AppShell) Select(id int) {
	s.Nav.Select(id)
}

// SetStatus updates the status bar text. Safe to call from any goroutine.
func (s *AppShell) SetStatus(text string) {
	fyne.Do(func() {
		s.Status.SetText(text)
	})
}
