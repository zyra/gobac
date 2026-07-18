package ui

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/session"
	"github.com/zyra/gobac/gui/internal/store"
)

// objectListProperty is the Object_List property id (BACnet property 76).
const objectListProperty = 76

// deviceObjectType is the BACnet object-type code for the Device object
// (types.ObjectTypeDevice == 8 in the library).
const deviceObjectType uint16 = 8

// maxLegacyDeviceInstance mirrors session's 16-bit legacy instance guard
// (see session/live.go's maxLegacyInstance). The browser checks a target
// device's own instance against it before issuing any read, so it can
// report the device-scoped status text the brief specifies; session's own
// guard (triggered per-object, inside ReadProperty/Write) reports a
// differently-worded, object-scoped message instead.
//
// TODO(L2): delete this guard once library task L2 (22-bit
// ReadObjectProperty/WriteObjectProperty) merges and session's fallback
// guard is retired — device ids above 65535 will then read normally.
const maxLegacyDeviceInstance = 65535

// objectTypeNames maps the 10 simulator-supported object types (plus the
// Device object type itself, needed to address Object_List) to their
// group-label / leaf-prefix text.
var objectTypeNames = map[uint16]string{
	0:  "analog-input",
	1:  "analog-output",
	2:  "analog-value",
	3:  "binary-input",
	4:  "binary-output",
	5:  "binary-value",
	8:  "device",
	13: "multi-state-input",
	14: "multi-state-output",
	19: "multi-state-value",
}

// objectTypeName returns the display name for an object type, falling back
// to "type-N" for types outside objectTypeNames.
func objectTypeName(t uint16) string {
	if name, ok := objectTypeNames[t]; ok {
		return name
	}
	return fmt.Sprintf("type-%d", t)
}

// wave1PropertyIDs is the fixed Wave-1 property set read into the property
// panel for a selected object, in display order.
var wave1PropertyIDs = []uint32{77, 28, 85, 117, 87, 111, 36, 81, 104}

// wave1PropertyNames maps a Wave-1 property id to its display name.
var wave1PropertyNames = map[uint32]string{
	77:  "Object_Name",
	28:  "Description",
	85:  "Present_Value",
	117: "Units",
	87:  "Priority_Array",
	111: "Status_Flags",
	36:  "Event_State",
	81:  "Out_Of_Service",
	104: "Relinquish_Default",
}

// propertyName returns the display name for a property id, falling back to
// "prop-N" for ids outside wave1PropertyNames.
func propertyName(id uint32) string {
	if name, ok := wave1PropertyNames[id]; ok {
		return name
	}
	return fmt.Sprintf("prop-%d", id)
}

// propertyColumns are the property table's column headers, in display
// order.
var propertyColumns = []string{"Property", "Value(s)", "Error"}

// writeTagOptions are the write dialog's selectable data tags, in display
// order — the CLI-supported set (map-cli-client "Supported Data Tags").
var writeTagOptions = []string{
	"Null(0)", "Boolean(1)", "Unsigned(2)", "Signed(3)",
	"Real(4)", "Double(5)", "CharacterString(7)", "Enumerated(9)",
}

// writeTagValues maps a writeTagOptions entry to its application tag.
var writeTagValues = map[string]uint8{
	"Null(0)": 0, "Boolean(1)": 1, "Unsigned(2)": 2, "Signed(3)": 3,
	"Real(4)": 4, "Double(5)": 5, "CharacterString(7)": 7, "Enumerated(9)": 9,
}

// BrowserView is the Object Browser navigation entry: an object tree (left)
// fed by a device's Object_List, and a property table (right) fed by
// Session.ReadMultiple for the selected object, with a write dialog.
//
// BrowserView is a proper widget (widget.BaseWidget + CreateRenderer)
// rather than an embedded *fyne.Container; see the identical note on
// DiscoveryView in discovery.go.
type BrowserView struct {
	widget.BaseWidget

	sess    session.Session
	objects *store.ObjectCache
	shell   *AppShell

	deviceKey  store.DeviceKey
	deviceAddr session.Address

	tree  *widget.Tree
	table *widget.Table

	refreshBtn *widget.Button
	writeBtn   *widget.Button

	selected     session.ObjectRef
	hasSelection bool
	propRows     []store.PropertyEntry

	// Write-dialog test seam: openWriteDialog populates these so tests can
	// set entry/select state and invoke submitWrite directly, instead of
	// driving the actual dialog widget through its confirm button.
	writeValueEntry    *widget.Entry
	writeTagSelect     *widget.Select
	writePriorityEntry *widget.Entry

	// loadDone/propsDone/writeDone are test-only synchronization seams,
	// mirroring DiscoveryView.sweepDone: if non-nil when the corresponding
	// method is invoked, each is closed once that call's background
	// goroutine finishes all of its work (including any fyne.Do UI
	// update). Production code leaves them nil, in which case the methods
	// don't touch them.
	loadDone  chan struct{}
	propsDone chan struct{}
	writeDone chan struct{}

	root *fyne.Container
}

// NewBrowserView builds the Object Browser view: an object tree bound to
// objects, and a property table loaded via sess.ReadMultiple for whichever
// object is selected in the tree.
func NewBrowserView(sess session.Session, objects *store.ObjectCache, shell *AppShell) fyne.CanvasObject {
	v := &BrowserView{
		sess:    sess,
		objects: objects,
		shell:   shell,
	}

	v.propRows = make([]store.PropertyEntry, len(wave1PropertyIDs))
	for i, id := range wave1PropertyIDs {
		v.propRows[i] = store.PropertyEntry{ID: id}
	}

	v.tree = widget.NewTree(v.childUIDs, v.isBranch, v.createTreeNode, v.updateTreeNode)
	v.tree.OnSelected = v.selectTreeNode

	v.table = widget.NewTable(
		func() (int, int) { return len(v.propRows), len(propertyColumns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(v.propertyCellText(id))
		},
	)
	v.table.ShowHeaderRow = true
	v.table.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	v.table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		obj.(*widget.Label).SetText(propertyColumns[id.Col])
	}

	v.refreshBtn = widget.NewButton("Refresh", v.refresh)
	v.writeBtn = widget.NewButton("Write…", v.openWriteDialog)

	toolbar := container.NewHBox(v.refreshBtn, v.writeBtn)
	right := container.NewBorder(toolbar, nil, nil, nil, v.table)

	split := container.NewHSplit(v.tree, right)
	split.Offset = 0.3

	v.root = container.NewBorder(nil, nil, nil, nil, split)
	v.ExtendBaseWidget(v)

	return v
}

// CreateRenderer implements fyne.Widget.
func (v *BrowserView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.root)
}

// treeUID scheme: "" is the root; "g:<type>" is a group (branch) node;
// "o:<type>:<instance>" is a leaf.

func groupUID(t uint16) string {
	return fmt.Sprintf("g:%d", t)
}

func parseGroupUID(uid string) (uint16, bool) {
	rest, ok := strings.CutPrefix(uid, "g:")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(rest, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(n), true
}

func leafUID(t uint16, instance uint32) string {
	return fmt.Sprintf("o:%d:%d", t, instance)
}

func parseLeafUID(uid string) (t uint16, instance uint32, ok bool) {
	rest, found := strings.CutPrefix(uid, "o:")
	if !found {
		return 0, 0, false
	}
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	typeNum, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, 0, false
	}
	instanceNum, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, 0, false
	}
	return uint16(typeNum), uint32(instanceNum), true
}

// childUIDs implements the tree's data source: the root's children are the
// distinct object-type groups present in the device's cached object list,
// sorted by type ascending; a group's children are that type's objects,
// sorted by instance ascending.
func (v *BrowserView) childUIDs(uid string) []string {
	entries := v.objects.Objects(v.deviceKey)

	if uid == "" {
		seen := make(map[uint16]bool)
		var types []uint16
		for _, e := range entries {
			if !seen[e.Type] {
				seen[e.Type] = true
				types = append(types, e.Type)
			}
		}
		sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
		out := make([]string, len(types))
		for i, t := range types {
			out[i] = groupUID(t)
		}
		return out
	}

	t, ok := parseGroupUID(uid)
	if !ok {
		return nil
	}
	var leaves []store.ObjectEntry
	for _, e := range entries {
		if e.Type == t {
			leaves = append(leaves, e)
		}
	}
	sort.Slice(leaves, func(i, j int) bool { return leaves[i].Instance < leaves[j].Instance })
	out := make([]string, len(leaves))
	for i, e := range leaves {
		out[i] = leafUID(e.Type, e.Instance)
	}
	return out
}

// isBranch reports whether uid is the root or a group node.
func (v *BrowserView) isBranch(uid string) bool {
	if uid == "" {
		return true
	}
	_, ok := parseGroupUID(uid)
	return ok
}

func (v *BrowserView) createTreeNode(bool) fyne.CanvasObject {
	return widget.NewLabel("")
}

func (v *BrowserView) updateTreeNode(uid string, branch bool, obj fyne.CanvasObject) {
	label := obj.(*widget.Label)
	if branch {
		t, ok := parseGroupUID(uid)
		if !ok {
			label.SetText("")
			return
		}
		label.SetText(objectTypeName(t))
		return
	}
	t, instance, ok := parseLeafUID(uid)
	if !ok {
		label.SetText("")
		return
	}
	label.SetText(fmt.Sprintf("%s %d", objectTypeName(t), instance))
}

// selectTreeNode loads the property panel for the selected leaf. Selecting
// a group node is a no-op.
func (v *BrowserView) selectTreeNode(uid string) {
	t, instance, ok := parseLeafUID(uid)
	if !ok {
		return
	}
	v.loadProperties(session.ObjectRef{Type: t, Instance: instance})
}

// LoadDevice loads dev's Object_List into the tree, replacing whatever
// device (if any) was previously loaded. Reads run on a goroutine; errors
// are reported via shell.SetStatus rather than returned.
func (v *BrowserView) LoadDevice(row store.DeviceRow) {
	v.deviceKey = row.Key
	v.deviceAddr = session.Address{IP: net.ParseIP(row.Key.IP)}
	v.hasSelection = false

	if row.Key.Instance > maxLegacyDeviceInstance {
		v.shell.SetStatus(fmt.Sprintf("device %d needs 22-bit support (pending L2)", row.Key.Instance))
		return
	}

	done := v.loadDone
	go func() {
		if done != nil {
			defer close(done)
		}

		obj := session.ObjectRef{Type: deviceObjectType, Instance: row.Key.Instance}
		values, err := v.sess.ReadProperty(context.Background(), v.deviceAddr, obj, objectListProperty)
		if err != nil {
			v.shell.SetStatus("object list read failed: " + err.Error())
			return
		}

		var entries []store.ObjectEntry
		for _, val := range values {
			if val.Tag != 12 {
				continue
			}
			ref, ok := val.Value.(session.ObjectRef)
			if !ok {
				continue
			}
			entries = append(entries, store.ObjectEntry{Type: ref.Type, Instance: ref.Instance})
		}
		v.objects.SetObjects(row.Key, entries)

		fyne.Do(func() { v.tree.Refresh() })
	}()
}

// readProperties reads the fixed Wave-1 property set for ref, one ReadSpec
// per property, and assembles the property table rows in wave1PropertyIDs
// order.
func (v *BrowserView) readProperties(ctx context.Context, ref session.ObjectRef) ([]store.PropertyEntry, error) {
	specs := make([]session.ReadSpec, len(wave1PropertyIDs))
	for i, pid := range wave1PropertyIDs {
		specs[i] = session.ReadSpec{Object: ref, Properties: []uint32{pid}}
	}

	results, err := v.sess.ReadMultiple(ctx, v.deviceAddr, specs)
	if err != nil {
		return nil, err
	}

	rows := make([]store.PropertyEntry, len(wave1PropertyIDs))
	for i, pid := range wave1PropertyIDs {
		entry := store.PropertyEntry{ID: pid}
		if i < len(results) {
			if propErr, ok := results[i].Errors[pid]; ok {
				entry.Err = propErr.Error()
			} else {
				entry.Values = results[i].Values
			}
		}
		rows[i] = entry
	}
	return rows, nil
}

// loadProperties loads the property panel for ref on a goroutine.
func (v *BrowserView) loadProperties(ref session.ObjectRef) {
	v.selected = ref
	v.hasSelection = true

	done := v.propsDone
	go func() {
		if done != nil {
			defer close(done)
		}

		rows, err := v.readProperties(context.Background(), ref)
		if err != nil {
			v.shell.SetStatus("property read failed: " + err.Error())
			return
		}

		v.objects.SetProperties(v.deviceKey, ref, rows)

		fyne.Do(func() {
			v.propRows = rows
			v.table.Refresh()
		})
	}()
}

// refresh re-reads the currently selected object's properties. A no-op if
// nothing is selected.
func (v *BrowserView) refresh() {
	if !v.hasSelection {
		return
	}
	v.loadProperties(v.selected)
}

// propertyCellText renders the data cell at id from v.propRows.
func (v *BrowserView) propertyCellText(id widget.TableCellID) string {
	if id.Row < 0 || id.Row >= len(v.propRows) {
		return ""
	}
	row := v.propRows[id.Row]
	switch id.Col {
	case 0:
		return propertyName(row.ID)
	case 1:
		if row.Err != "" {
			return ""
		}
		parts := make([]string, len(row.Values))
		for i, val := range row.Values {
			parts[i] = FormatValue(val)
		}
		return strings.Join(parts, ", ")
	case 2:
		return row.Err
	}
	return ""
}

// validateWritePriority validates the write dialog's priority entry: blank
// or "0" means no explicit priority; otherwise it must be an integer in
// [1, 16], and 6 (reserved) is rejected with an exact message.
func validateWritePriority(text string) error {
	text = strings.TrimSpace(text)
	if text == "" || text == "0" {
		return nil
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("priority must be a number")
	}
	if n == 6 {
		return fmt.Errorf("priority 6 is reserved")
	}
	if n < 1 || n > 16 {
		return fmt.Errorf("priority must be between 1 and 16")
	}
	return nil
}

// parseWritePriority parses the write dialog's priority entry into a
// WriteRequest.Priority value, treating blank or "0" as "no priority".
// Callers must run validateWritePriority first.
func parseWritePriority(text string) (uint8, error) {
	text = strings.TrimSpace(text)
	if text == "" || text == "0" {
		return 0, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("priority must be a number")
	}
	return uint8(n), nil
}

// openWriteDialog opens the write dialog for the currently selected
// object. A no-op (with a status message) if nothing is selected.
func (v *BrowserView) openWriteDialog() {
	if !v.hasSelection {
		v.shell.SetStatus("select an object before writing")
		return
	}

	v.writeValueEntry = widget.NewEntry()
	v.writeTagSelect = widget.NewSelect(writeTagOptions, nil)
	v.writeTagSelect.SetSelected(writeTagOptions[4]) // Real(4): the common Present_Value tag
	v.writePriorityEntry = widget.NewEntry()
	v.writePriorityEntry.SetText("0")
	v.writePriorityEntry.Validator = validateWritePriority

	form := widget.NewForm(
		widget.NewFormItem("Value", v.writeValueEntry),
		widget.NewFormItem("Tag", v.writeTagSelect),
		widget.NewFormItem("Priority", v.writePriorityEntry),
	)

	// BrowserView is constructed without a fyne.Window reference (see
	// NewBrowserView); the app has exactly one window in Wave-1, so it is
	// resolved here the same way fyne's own menu handling does.
	windows := fyne.CurrentApp().Driver().AllWindows()
	if len(windows) == 0 {
		return
	}

	dialog.NewCustomConfirm("Write…", "Write", "Cancel", form, func(ok bool) {
		if ok {
			v.submitWrite()
		}
	}, windows[0]).Show()
}

// submitWrite validates and parses the write dialog's current entry state
// and issues the write on a goroutine, refreshing the property panel on
// success.
func (v *BrowserView) submitWrite() {
	if !v.hasSelection || v.writeTagSelect == nil {
		return
	}

	tag, ok := writeTagValues[v.writeTagSelect.Selected]
	if !ok {
		v.shell.SetStatus("select a data tag")
		return
	}
	if err := validateWritePriority(v.writePriorityEntry.Text); err != nil {
		v.shell.SetStatus(err.Error())
		return
	}
	priority, err := parseWritePriority(v.writePriorityEntry.Text)
	if err != nil {
		v.shell.SetStatus(err.Error())
		return
	}
	value, err := ParseWriteValue(tag, v.writeValueEntry.Text)
	if err != nil {
		v.shell.SetStatus("invalid value: " + err.Error())
		return
	}

	ref := v.selected
	req := session.WriteRequest{Tag: tag, Priority: priority, Value: value}

	done := v.writeDone
	go func() {
		if done != nil {
			defer close(done)
		}

		if err := v.sess.Write(context.Background(), v.deviceAddr, ref, req); err != nil {
			v.shell.SetStatus("write failed: " + err.Error())
			return
		}

		rows, err := v.readProperties(context.Background(), ref)
		if err != nil {
			v.shell.SetStatus("property read failed: " + err.Error())
			return
		}
		v.objects.SetProperties(v.deviceKey, ref, rows)

		fyne.Do(func() {
			v.propRows = rows
			v.table.Refresh()
		})
	}()
}
