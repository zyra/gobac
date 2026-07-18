package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
)

// discoveryColumns are the discovery table's column headers, in display
// order.
var discoveryColumns = []string{
	"Instance", "Name", "Address", "Vendor ID", "Max APDU", "Segmentation", "Source", "Last seen",
}

// segmentationText maps types.Segmentation values (bacnet/types/segmentation.go)
// to their display text.
var segmentationText = map[uint8]string{
	0: "both",
	1: "transmit",
	2: "receive",
	3: "none",
}

// sourceText maps store.DeviceRow.Source values to the plain-language
// display text the Source column renders (task U3): "Simulated" for a
// device injected by the Simulator view's Run controls, "Network" for a
// real Who-Is sighting.
var sourceText = map[string]string{
	"simulated": "Simulated",
	"network":   "Network",
}

// defaultSweepDuration is the pre-selected entry in the duration selector.
const defaultSweepDuration = "3s"

// sweepDurations are the selectable Who-Is sweep durations, in display
// order. Every entry must parse with time.ParseDuration.
var sweepDurations = []string{"1s", "3s", "10s"}

// DiscoveryView is the Discovery navigation entry: sweep controls plus a
// live device table bound to a store.DeviceStore.
//
// DiscoveryView is a proper widget (widget.BaseWidget + CreateRenderer)
// rather than an embedded *fyne.Container, for the same reason AppShell is
// (see shell.go): a struct that only embeds *fyne.Container satisfies
// fyne.CanvasObject by promotion, but the driver's render-tree walk
// recognizes concrete *fyne.Container or fyne.Widget values, not types that
// merely embed one — so its children would never be painted when placed
// inside another container (e.g. AppShell.Content).
type DiscoveryView struct {
	widget.BaseWidget

	sess    session.Session
	devices *store.DeviceStore
	shell   *AppShell

	sweepBtn *widget.Button
	clearBtn *widget.Button
	duration *widget.Select
	count    *widget.Label
	table    *widget.Table

	cached []store.DeviceRow

	// OnSelect is invoked with the selected row whenever the user picks a
	// table row. It is nil-safe: OnSelect may be left unset.
	OnSelect func(store.DeviceRow)

	removeListener func()

	root *fyne.Container

	// sweepDone is a test-only synchronization seam: if non-nil when
	// sweep() is invoked, it is closed once that sweep's background
	// goroutine finishes all of its work (including re-enabling
	// sweepBtn and setting the status). Production code leaves it nil,
	// in which case sweep() does not touch it. Tests set a fresh channel
	// before each Tap so they can block on <-sweepDone instead of racing
	// on widget state from a second goroutine.
	sweepDone chan struct{}
}

// NewDiscoveryView builds the Discovery view: a toolbar (sweep, duration,
// clear, device count) above a live device table fed by devices and swept
// via sess.Discover.
func NewDiscoveryView(sess session.Session, devices *store.DeviceStore, shell *AppShell) fyne.CanvasObject {
	v := &DiscoveryView{
		sess:    sess,
		devices: devices,
		shell:   shell,
		count:   widget.NewLabel("0 devices"),
	}

	v.duration = widget.NewSelect(sweepDurations, nil)
	v.duration.SetSelected(defaultSweepDuration)

	v.sweepBtn = widget.NewButton("Sweep", v.sweep)
	v.clearBtn = widget.NewButton("Clear", v.clear)

	toolbar := container.NewHBox(v.sweepBtn, v.duration, v.clearBtn, v.count)

	v.table = widget.NewTable(
		func() (int, int) { return len(v.cached), len(discoveryColumns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(v.cellText(id))
		},
	)
	v.table.ShowHeaderRow = true
	v.table.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	v.table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		obj.(*widget.Label).SetText(discoveryColumns[id.Col])
	}
	v.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Row >= len(v.cached) {
			return
		}
		if v.OnSelect != nil {
			v.OnSelect(v.cached[id.Row])
		}
	}

	v.root = container.NewBorder(toolbar, nil, nil, nil, v.table)
	v.ExtendBaseWidget(v)

	v.removeListener = v.devices.AddListener(v.refresh)
	v.refresh()

	return v
}

// CreateRenderer implements fyne.Widget.
func (v *DiscoveryView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.root)
}

// cellText renders the data cell at id from the cached snapshot.
func (v *DiscoveryView) cellText(id widget.TableCellID) string {
	if id.Row < 0 || id.Row >= len(v.cached) {
		return ""
	}
	row := v.cached[id.Row]
	switch id.Col {
	case 0:
		return fmt.Sprintf("%d", row.Key.Instance)
	case 1:
		return row.Name
	case 2:
		return fmt.Sprintf("%s:%d", row.Key.IP, row.Port)
	case 3:
		return fmt.Sprintf("%d", row.VendorID)
	case 4:
		return fmt.Sprintf("%d", row.MaxApdu)
	case 5:
		if text, ok := segmentationText[row.Segmentation]; ok {
			return text
		}
		return fmt.Sprintf("%d", row.Segmentation)
	case 6:
		if text, ok := sourceText[row.Source]; ok {
			return text
		}
		return row.Source
	case 7:
		return row.LastSeen.Format("15:04:05")
	}
	return ""
}

// refresh re-reads the store snapshot and updates the table and count
// label. Safe to call from any goroutine.
func (v *DiscoveryView) refresh() {
	fyne.Do(func() {
		v.cached = v.devices.Snapshot()
		v.table.Refresh()
		v.count.SetText(fmt.Sprintf("%d devices", len(v.cached)))
	})
}

// clear empties the device store; the store listener drives the table/count
// refresh.
func (v *DiscoveryView) clear() {
	v.devices.Clear()
}

// sweep runs a Who-Is sweep for the selected duration on a goroutine,
// upserting each discovered device into the store as it arrives.
func (v *DiscoveryView) sweep() {
	duration, err := time.ParseDuration(v.duration.Selected)
	if err != nil {
		duration = 3 * time.Second
	}

	v.sweepBtn.Disable()

	done := v.sweepDone
	go func() {
		if done != nil {
			defer close(done)
		}

		ch, err := v.sess.Discover(context.Background(), duration)
		if err != nil {
			fyne.Do(func() { v.sweepBtn.Enable() })
			v.shell.SetStatus("sweep failed: " + err.Error())
			return
		}

		for summary := range ch {
			v.devices.Upsert(store.DeviceRow{
				Key: store.DeviceKey{
					Instance: summary.Instance,
					IP:       summary.IP.String(),
				},
				Port:         summary.Port,
				VendorID:     summary.VendorID,
				MaxApdu:      summary.MaxApdu,
				Segmentation: uint8(summary.Segmentation),
				Source:       "network",
			})
		}

		fyne.Do(func() { v.sweepBtn.Enable() })
		v.shell.SetStatus(fmt.Sprintf("sweep complete: %d devices", v.devices.Len()))
	}()
}
