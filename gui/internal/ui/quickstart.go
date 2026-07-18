package ui

import (
	"bytes"
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/assets"
	"github.com/zyra/gobac/gui/internal/simrun"
	"github.com/zyra/gobac/gui/internal/store"
	"github.com/zyra/gobac/v2/simulator"
)

// quickstartDescription is the Quickstart view's static explanatory text.
const quickstartDescription = "Run a bundled demo scenario in-process on loopback UDP — no external processes. Its devices appear in Discovery for browsing, reading, and writing over the real client path."

// quickstartRunner is the subset of *simrun.Runner the Quickstart view
// depends on, so tests can substitute a fake instead of running real UDP
// sockets.
type quickstartRunner interface {
	Devices() []simrun.RunningDevice
	Stop()
	Err() <-chan error
}

var _ quickstartRunner = (*simrun.Runner)(nil)

// QuickstartView is the Quickstart navigation entry (task G8): a one-click
// in-process simulator run whose devices are injected into the shared
// DeviceStore (Source "local-sim") so Discovery and the Object Browser can
// exercise them over real loopback UDP.
//
// QuickstartView is a proper widget (widget.BaseWidget + CreateRenderer)
// rather than an embedded *fyne.Container; see the identical note on
// DiscoveryView in discovery.go.
type QuickstartView struct {
	widget.BaseWidget

	devices *store.DeviceStore
	shell   *AppShell

	startBtn *widget.Button
	stopBtn  *widget.Button
	list     *widget.List

	running quickstartRunner
	rows    []simrun.RunningDevice

	errWatchStop chan struct{}

	// StartRunner starts a decoded scenario. Exported so tests can replace
	// it with a fake runner instead of exercising real UDP sockets.
	// Defaults to wrapping simrun.Start.
	StartRunner func(ctx context.Context, sc *simulator.Scenario) (quickstartRunner, error)

	// startDone/stopDone are test-only synchronization seams, mirroring
	// DiscoveryView.sweepDone: if non-nil when the corresponding method is
	// invoked, each is closed once that call's background goroutine
	// finishes all of its work (including any fyne.Do UI update).
	// Production code leaves them nil.
	startDone chan struct{}
	stopDone  chan struct{}

	root *fyne.Container
}

// NewQuickstartView builds the Quickstart view.
func NewQuickstartView(devices *store.DeviceStore, shell *AppShell) fyne.CanvasObject {
	v := &QuickstartView{
		devices:     devices,
		shell:       shell,
		StartRunner: defaultStartRunner,
	}

	description := widget.NewLabel(quickstartDescription)
	description.Wrapping = fyne.TextWrapWord

	v.startBtn = widget.NewButton("Start local simulator", v.start)
	v.stopBtn = widget.NewButton("Stop", v.stop)
	v.stopBtn.Disable()

	toolbar := container.NewHBox(v.startBtn, v.stopBtn)

	v.list = widget.NewList(
		func() int { return len(v.rows) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(v.rowText(id))
		},
	)

	v.root = container.NewBorder(
		container.NewVBox(description, toolbar), nil, nil, nil, v.list,
	)
	v.ExtendBaseWidget(v)

	return v
}

// CreateRenderer implements fyne.Widget.
func (v *QuickstartView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.root)
}

// defaultStartRunner decodes sc and starts it via simrun.Start, adapting
// *simrun.Runner to the quickstartRunner interface.
func defaultStartRunner(ctx context.Context, sc *simulator.Scenario) (quickstartRunner, error) {
	return simrun.Start(ctx, sc)
}

// rowText renders the running-device list entry at id.
func (v *QuickstartView) rowText(id widget.ListItemID) string {
	if id < 0 || id >= len(v.rows) {
		return ""
	}
	d := v.rows[id]
	return fmt.Sprintf("%d %s — %s:%d", d.ID, d.Name, d.Addr, d.Port)
}

// start decodes the bundled quickstart scenario, starts it, and injects its
// devices into the shared DeviceStore as Source "local-sim" rows.
func (v *QuickstartView) start() {
	v.startBtn.Disable()

	done := v.startDone
	go func() {
		if done != nil {
			defer close(done)
		}

		sc, err := simulator.DecodeScenario(bytes.NewReader(assets.QuickstartScenario), "yaml")
		if err != nil {
			fyne.Do(func() { v.startBtn.Enable() })
			v.shell.SetStatus("quickstart scenario invalid: " + err.Error())
			return
		}

		r, err := v.StartRunner(context.Background(), sc)
		if err != nil {
			fyne.Do(func() { v.startBtn.Enable() })
			v.shell.SetStatus("quickstart start failed: " + err.Error())
			return
		}

		rows := r.Devices()
		for _, d := range rows {
			v.devices.Upsert(store.DeviceRow{
				Key:    store.DeviceKey{Instance: d.ID, IP: d.Addr},
				Port:   d.Port,
				Source: "local-sim",
			})
		}

		v.errWatchStop = make(chan struct{})
		go v.watchErrors(r, v.errWatchStop)

		fyne.Do(func() {
			v.running = r
			v.rows = rows
			v.list.Refresh()
			v.startBtn.Disable()
			v.stopBtn.Enable()
		})
		v.shell.SetStatus(fmt.Sprintf("local simulator running (%d devices)", len(rows)))
	}()
}

// watchErrors forwards fatal runner errors to the status bar until stop is
// closed.
func (v *QuickstartView) watchErrors(r quickstartRunner, stop <-chan struct{}) {
	for {
		select {
		case err := <-r.Err():
			v.shell.SetStatus("simulator error: " + err.Error())
		case <-stop:
			return
		}
	}
}

// stop shuts the running simulator down, removes its devices from the
// DeviceStore, and resets the buttons.
func (v *QuickstartView) stop() {
	v.stopBtn.Disable()

	r := v.running
	if r == nil {
		fyne.Do(func() { v.startBtn.Enable() })
		return
	}

	done := v.stopDone
	go func() {
		if done != nil {
			defer close(done)
		}

		if v.errWatchStop != nil {
			close(v.errWatchStop)
			v.errWatchStop = nil
		}
		r.Stop()

		for _, d := range v.rows {
			v.devices.Remove(store.DeviceKey{Instance: d.ID, IP: d.Addr})
		}
		fyne.Do(func() {
			v.running = nil
			v.rows = nil
			v.list.Refresh()
			v.startBtn.Enable()
			v.stopBtn.Disable()
		})
		v.shell.SetStatus("local simulator stopped")
	}()
}
