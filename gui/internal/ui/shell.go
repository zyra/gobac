// Package ui contains the Fyne views and application shell for the GoBAC
// Workstation. It is the only package in this module that imports Fyne.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// navLabels are the left-navigation entries, in display order.
var navLabels = []string{"Discovery", "Object Browser", "Simulator Editor", "Quickstart"}

// viewLabels are the placeholder center-content texts, one per navLabels
// entry at the same index.
var viewLabels = []string{"Discovery view", "Object browser", "Scenario editor", "Quickstart"}

// AppShell is the top-level content for the GoBAC Workstation main window:
// a left navigation list, a center content stack that switches per
// selection, and a bottom status bar.
type AppShell struct {
	*fyne.Container

	Nav     *widget.List
	Content *fyne.Container
	Status  *widget.Label
}

// NewAppShell builds the application shell.
func NewAppShell(a fyne.App, w fyne.Window) *AppShell {
	shell := &AppShell{
		Status: widget.NewLabel(""),
	}

	views := make([]fyne.CanvasObject, len(viewLabels))
	for i, text := range viewLabels {
		views[i] = widget.NewLabel(text)
	}
	shell.Content = container.NewStack(views...)
	shell.selectView(0)

	shell.Nav = widget.NewList(
		func() int { return len(navLabels) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		updateNavItem,
	)
	shell.Nav.OnSelected = func(id widget.ListItemID) {
		shell.selectView(id)
	}

	statusBar := container.NewHBox(shell.Status)

	shell.Container = container.NewBorder(nil, statusBar, shell.Nav, nil, shell.Content)

	return shell
}

// updateNavItem renders the navigation row at id into obj, an item created
// by the List's CreateItem callback.
func updateNavItem(id widget.ListItemID, obj fyne.CanvasObject) {
	obj.(*widget.Label).SetText(navLabels[id])
}

// selectView shows the content view at index id and hides all others.
func (s *AppShell) selectView(id int) {
	for i, obj := range s.Content.Objects {
		if i == id {
			obj.Show()
		} else {
			obj.Hide()
		}
	}
}

// SetStatus updates the status bar text. Safe to call from any goroutine.
func (s *AppShell) SetStatus(text string) {
	fyne.Do(func() {
		s.Status.SetText(text)
	})
}
