package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/zyra/gobac/gui/internal/scenariodoc"
	"github.com/zyra/gobac/v2/simulator"
)

// networkModes are the network form's Mode select options, in display
// order — the exact three modes simulator.Scenario.Validate accepts.
var networkModes = []string{"single-device", "multi-ip", "multi-port"}

// objectTypeOptions are the object type picker's options, in display order —
// the 9 scenario object types scenariodoc.AddObject recognizes.
var objectTypeOptions = []string{
	"analog-input", "analog-output", "analog-value",
	"binary-input", "binary-output", "binary-value",
	"multi-state-input", "multi-state-output", "multi-state-value",
}

// binaryPresentValueOptions are the Present Value / Relinquish Default
// select options shown for binary object types.
var binaryPresentValueOptions = []string{"inactive", "active"}

// classifyObjectType groups a scenario object type name (any spelling
// scenariodoc.AddObject accepts) into "analog", "binary", or "multistate",
// or "" if it is not one of the 9 scenario object types. It mirrors
// scenariodoc's own (unexported) canonicalObjectType classification just
// closely enough to pick the right value editor widget.
func classifyObjectType(t string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(t), "_", "-")) {
	case "analog-input", "analog-output", "analog-value":
		return "analog"
	case "binary-input", "binary-output", "binary-value":
		return "binary"
	case "multistate-input", "multistate-output", "multistate-value",
		"multi-state-input", "multi-state-output", "multi-state-value":
		return "multistate"
	default:
		return ""
	}
}

// EditorView is the Simulator Editor navigation entry: toolbar (New, Open,
// Save, Save As), a network form, and master-detail device/object editing
// over a scenariodoc.Document, with live validation.
//
// EditorView is a proper widget (widget.BaseWidget + CreateRenderer) rather
// than an embedded *fyne.Container; see the identical note on DiscoveryView
// in discovery.go.
type EditorView struct {
	widget.BaseWidget

	shell *AppShell
	doc   *scenariodoc.Document

	// fieldErrors is the most recent scenariodoc.FieldErrors result, kept
	// for both the field-hint display and same-package test access.
	fieldErrors map[string]string

	titleLabel   *widget.Label
	summaryLabel *widget.Label
	saveBtn      *widget.Button

	modeSelect *widget.Select
	ifaceEntry *widget.Entry
	portEntry  *widget.Entry

	deviceList      *widget.List
	addDeviceBtn    *widget.Button
	removeDeviceBtn *widget.Button
	selectedDevice  int // -1 = none selected

	deviceFormBox      *fyne.Container
	devIDEntry         *widget.Entry
	devNameEntry       *widget.Entry
	devAddressEntry    *widget.Entry
	devPortEntry       *widget.Entry
	devVendorIDEntry   *widget.Entry
	devVendorNameEntry *widget.Entry
	devModelNameEntry  *widget.Entry
	devProtoRevEntry   *widget.Entry

	objTypeSelect   *widget.Select
	objectList      *widget.List
	addObjectBtn    *widget.Button
	removeObjectBtn *widget.Button
	selectedObject  int // -1 = none selected

	objectFormBox           *fyne.Container
	objInstanceEntry        *widget.Entry
	objNameEntry            *widget.Entry
	objDescEntry            *widget.Entry
	objPresentValueEntry    *widget.Entry
	objPresentValueSelect   *widget.Select
	objUnitsEntry           *widget.Entry
	objWritableCheck        *widget.Check
	objCommandableCheck     *widget.Check
	objRelinquishEntry      *widget.Entry
	objRelinquishSelect     *widget.Select
	objInitialPriorityEntry *widget.Entry
	objNumberOfStatesEntry  *widget.Entry
	objCovIncrementEntry    *widget.Entry

	root *fyne.Container
}

// NewEditorView builds the Simulator Editor view, holding a
// *scenariodoc.Document that starts as scenariodoc.New() (a minimal,
// already-valid single-device scenario).
func NewEditorView(shell *AppShell) fyne.CanvasObject {
	v := &EditorView{
		shell:          shell,
		doc:            scenariodoc.New(),
		selectedDevice: -1,
		selectedObject: -1,
	}
	v.buildWidgets()
	v.ExtendBaseWidget(v)
	v.selectDevice(0)
	v.revalidate()
	return v
}

// CreateRenderer implements fyne.Widget.
func (v *EditorView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.root)
}

// buildWidgets constructs every widget that lives for the view's whole
// lifetime (the toolbar, network form, and the two list widgets). The
// device/object detail forms are rebuilt per-selection by
// rebuildDeviceForm/rebuildObjectForm.
func (v *EditorView) buildWidgets() {
	newBtn := widget.NewButton("New", v.onNew)
	openBtn := widget.NewButton("Open", v.onOpen)
	v.saveBtn = widget.NewButton("Save", v.onSave)
	saveAsBtn := widget.NewButton("Save As", v.onSaveAs)
	toolbar := container.NewHBox(newBtn, openBtn, v.saveBtn, saveAsBtn)

	v.titleLabel = widget.NewLabel("")
	v.summaryLabel = widget.NewLabel("valid")

	v.modeSelect = widget.NewSelect(networkModes, nil)
	v.modeSelect.SetSelected(v.doc.Scenario().Network.Mode)
	v.modeSelect.OnChanged = v.onModeChanged

	v.ifaceEntry = widget.NewEntry()
	v.ifaceEntry.SetText(v.doc.Scenario().Network.Interface)
	v.ifaceEntry.OnChanged = v.onIfaceChanged

	v.portEntry = widget.NewEntry()
	v.portEntry.SetPlaceHolder("47808")
	if port := v.doc.Scenario().Network.Port; port != 0 {
		v.portEntry.SetText(strconv.FormatUint(uint64(port), 10))
	}
	v.portEntry.OnChanged = v.onNetworkPortChanged

	networkForm := widget.NewForm(
		widget.NewFormItem("Mode", v.modeSelect),
		widget.NewFormItem("Interface", v.ifaceEntry),
		widget.NewFormItem("Port", v.portEntry),
	)

	v.deviceList = widget.NewList(v.deviceListLength, v.deviceListCreate, v.deviceListUpdate)
	v.deviceList.OnSelected = func(id widget.ListItemID) { v.selectDevice(id) }
	v.addDeviceBtn = widget.NewButton("Add", v.onAddDevice)
	v.removeDeviceBtn = widget.NewButton("Remove", v.onRemoveDevice)
	deviceListBox := container.NewBorder(
		container.NewHBox(v.addDeviceBtn, v.removeDeviceBtn), nil, nil, nil, v.deviceList)

	v.objTypeSelect = widget.NewSelect(objectTypeOptions, nil)
	v.objTypeSelect.SetSelected(objectTypeOptions[0])
	v.objectList = widget.NewList(v.objectListLength, v.objectListCreate, v.objectListUpdate)
	v.objectList.OnSelected = func(id widget.ListItemID) { v.selectObject(id) }
	v.addObjectBtn = widget.NewButton("Add", v.onAddObject)
	v.removeObjectBtn = widget.NewButton("Remove", v.onRemoveObject)
	objectListBox := container.NewBorder(
		container.NewHBox(v.objTypeSelect, v.addObjectBtn, v.removeObjectBtn), nil, nil, nil, v.objectList)

	listSplit := container.NewVSplit(deviceListBox, objectListBox)

	v.deviceFormBox = container.NewVBox()
	v.objectFormBox = container.NewVBox()
	formSplit := container.NewVSplit(v.deviceFormBox, v.objectFormBox)

	mainSplit := container.NewHSplit(listSplit, formSplit)

	top := container.NewVBox(toolbar, v.titleLabel, networkForm)
	v.root = container.NewBorder(top, v.summaryLabel, nil, nil, mainSplit)
}

// ---- device list ----

func (v *EditorView) deviceListLength() int {
	return len(v.doc.Scenario().Devices)
}

func (v *EditorView) deviceListCreate() fyne.CanvasObject {
	return widget.NewLabel("")
}

func (v *EditorView) deviceListUpdate(id widget.ListItemID, obj fyne.CanvasObject) {
	obj.(*widget.Label).SetText(v.deviceCellText(id))
}

func (v *EditorView) deviceCellText(id widget.ListItemID) string {
	devices := v.doc.Scenario().Devices
	if id < 0 || int(id) >= len(devices) {
		return ""
	}
	d := devices[id]
	return fmt.Sprintf("%d — %s", d.ID, d.Name)
}

// ---- object list ----

func (v *EditorView) objectListLength() int {
	dev := v.currentDevice()
	if dev == nil {
		return 0
	}
	return len(dev.Objects)
}

func (v *EditorView) objectListCreate() fyne.CanvasObject {
	return widget.NewLabel("")
}

func (v *EditorView) objectListUpdate(id widget.ListItemID, obj fyne.CanvasObject) {
	obj.(*widget.Label).SetText(v.objectCellText(id))
}

func (v *EditorView) objectCellText(id widget.ListItemID) string {
	dev := v.currentDevice()
	if dev == nil || id < 0 || int(id) >= len(dev.Objects) {
		return ""
	}
	o := dev.Objects[id]
	return fmt.Sprintf("%s %d — %s", o.Type, o.Instance, o.Name)
}

// ---- selection helpers ----

// currentDevice returns a pointer into the live Devices slice for the
// current selection, or nil if none/out of range.
func (v *EditorView) currentDevice() *simulator.DeviceSpec {
	devices := v.doc.Scenario().Devices
	if v.selectedDevice < 0 || v.selectedDevice >= len(devices) {
		return nil
	}
	return &devices[v.selectedDevice]
}

// currentObject returns a pointer into the live Objects slice of the
// currently selected device, or nil if none/out of range.
func (v *EditorView) currentObject() *simulator.ObjectSpec {
	dev := v.currentDevice()
	if dev == nil {
		return nil
	}
	if v.selectedObject < 0 || v.selectedObject >= len(dev.Objects) {
		return nil
	}
	return &dev.Objects[v.selectedObject]
}

// selectDevice switches the detail forms to device idx (-1 for none),
// rebuilding both the device and (now-cleared) object forms and repainting
// field-error hints on the freshly built widgets — needed because
// rebuildDeviceForm/rebuildObjectForm construct brand new Entry/Select
// widgets that start with no validation hint applied.
func (v *EditorView) selectDevice(idx int) {
	v.selectedDevice = idx
	v.selectedObject = -1
	v.rebuildDeviceForm()
	v.objectList.Refresh()
	v.rebuildObjectForm()
	v.revalidate()
}

// selectObject switches the object detail form to object idx (-1 for
// none) on the currently selected device; see selectDevice for why this
// also revalidates.
func (v *EditorView) selectObject(idx int) {
	v.selectedObject = idx
	v.rebuildObjectForm()
	v.revalidate()
}

// ---- device add/remove ----

func (v *EditorView) onAddDevice() {
	v.doc.AddDevice()
	v.deviceList.Refresh()
	idx := len(v.doc.Scenario().Devices) - 1
	v.selectDevice(idx)
	v.deviceList.Select(idx)
	v.revalidate()
}

func (v *EditorView) onRemoveDevice() {
	if v.selectedDevice < 0 {
		return
	}
	v.doc.RemoveDevice(v.selectedDevice)
	v.deviceList.Refresh()
	next := v.selectedDevice
	if next >= len(v.doc.Scenario().Devices) {
		next = len(v.doc.Scenario().Devices) - 1
	}
	v.selectDevice(next)
	if next >= 0 {
		v.deviceList.Select(next)
	}
	v.revalidate()
}

// ---- object add/remove ----

func (v *EditorView) onAddObject() {
	if v.selectedDevice < 0 || v.objTypeSelect == nil {
		return
	}
	spec := v.doc.AddObject(v.selectedDevice, v.objTypeSelect.Selected)
	if spec == nil {
		return
	}
	v.objectList.Refresh()
	idx := len(v.doc.Scenario().Devices[v.selectedDevice].Objects) - 1
	v.selectObject(idx)
	v.objectList.Select(idx)
	v.revalidate()
}

func (v *EditorView) onRemoveObject() {
	if v.selectedDevice < 0 || v.selectedObject < 0 {
		return
	}
	v.doc.RemoveObject(v.selectedDevice, v.selectedObject)
	v.objectList.Refresh()
	objs := v.doc.Scenario().Devices[v.selectedDevice].Objects
	next := v.selectedObject
	if next >= len(objs) {
		next = len(objs) - 1
	}
	v.selectObject(next)
	if next >= 0 {
		v.objectList.Select(next)
	}
	v.revalidate()
}

// ---- network form ----

func (v *EditorView) onModeChanged(mode string) {
	v.doc.Scenario().Network.Mode = mode
	v.doc.MarkDirty()
	v.applyAddressEnablement()
	v.revalidate()
}

func (v *EditorView) onIfaceChanged(text string) {
	v.doc.Scenario().Network.Interface = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onNetworkPortChanged(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		v.doc.Scenario().Network.Port = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return
	}
	v.doc.Scenario().Network.Port = uint16(n)
	v.doc.MarkDirty()
	v.revalidate()
}

// refreshNetworkForm re-displays the network form fields from the current
// document without re-triggering their commit handlers (used after New/Open
// replace the whole document).
func (v *EditorView) refreshNetworkForm() {
	net := v.doc.Scenario().Network
	setSelectSilently(v.modeSelect, net.Mode)
	setEntrySilently(v.ifaceEntry, net.Interface)
	if net.Port != 0 {
		setEntrySilently(v.portEntry, strconv.FormatUint(uint64(net.Port), 10))
	} else {
		setEntrySilently(v.portEntry, "")
	}
	v.applyAddressEnablement()
}

// applyAddressEnablement enables the device Address entry only in
// multi-ip mode, per the brief; otherwise it is disabled with a
// "(from network)" placeholder.
func (v *EditorView) applyAddressEnablement() {
	if v.devAddressEntry == nil {
		return
	}
	if v.doc.Scenario().Network.Mode == "multi-ip" {
		v.devAddressEntry.Enable()
		v.devAddressEntry.SetPlaceHolder("")
	} else {
		v.devAddressEntry.Disable()
		v.devAddressEntry.SetPlaceHolder("(from network)")
	}
}

// ---- device form ----

func (v *EditorView) rebuildDeviceForm() {
	dev := v.currentDevice()
	if dev == nil {
		v.deviceFormBox.Objects = nil
		v.devIDEntry, v.devNameEntry, v.devAddressEntry, v.devPortEntry = nil, nil, nil, nil
		v.devVendorIDEntry, v.devVendorNameEntry, v.devModelNameEntry, v.devProtoRevEntry = nil, nil, nil, nil
		v.deviceFormBox.Refresh()
		return
	}

	v.devIDEntry = widget.NewEntry()
	v.devIDEntry.SetText(strconv.FormatUint(uint64(dev.ID), 10))
	v.devIDEntry.OnChanged = v.onDeviceIDChanged

	v.devNameEntry = widget.NewEntry()
	v.devNameEntry.SetText(dev.Name)
	v.devNameEntry.OnChanged = v.onDeviceNameChanged

	v.devAddressEntry = widget.NewEntry()
	v.devAddressEntry.SetText(dev.Address)
	v.devAddressEntry.OnChanged = v.onDeviceAddressChanged
	v.applyAddressEnablement()

	v.devPortEntry = widget.NewEntry()
	if dev.Port != 0 {
		v.devPortEntry.SetText(strconv.FormatUint(uint64(dev.Port), 10))
	}
	v.devPortEntry.OnChanged = v.onDevicePortChanged

	v.devVendorIDEntry = widget.NewEntry()
	if dev.VendorID != 0 {
		v.devVendorIDEntry.SetText(strconv.FormatUint(uint64(dev.VendorID), 10))
	}
	v.devVendorIDEntry.OnChanged = v.onDeviceVendorIDChanged

	v.devVendorNameEntry = widget.NewEntry()
	v.devVendorNameEntry.SetPlaceHolder("GoBAC")
	v.devVendorNameEntry.SetText(dev.VendorName)
	v.devVendorNameEntry.OnChanged = v.onDeviceVendorNameChanged

	v.devModelNameEntry = widget.NewEntry()
	v.devModelNameEntry.SetPlaceHolder("GoBAC Simulator")
	v.devModelNameEntry.SetText(dev.ModelName)
	v.devModelNameEntry.OnChanged = v.onDeviceModelNameChanged

	v.devProtoRevEntry = widget.NewEntry()
	v.devProtoRevEntry.SetPlaceHolder("14")
	if dev.ProtocolRevision != 0 {
		v.devProtoRevEntry.SetText(strconv.FormatUint(uint64(dev.ProtocolRevision), 10))
	}
	v.devProtoRevEntry.OnChanged = v.onDeviceProtocolRevisionChanged

	form := widget.NewForm(
		widget.NewFormItem("ID", v.devIDEntry),
		widget.NewFormItem("Name", v.devNameEntry),
		widget.NewFormItem("Address", v.devAddressEntry),
		widget.NewFormItem("Port", v.devPortEntry),
		widget.NewFormItem("Vendor ID", v.devVendorIDEntry),
		widget.NewFormItem("Vendor Name", v.devVendorNameEntry),
		widget.NewFormItem("Model Name", v.devModelNameEntry),
		widget.NewFormItem("Protocol Revision", v.devProtoRevEntry),
	)
	v.deviceFormBox.Objects = []fyne.CanvasObject{form}
	v.deviceFormBox.Refresh()
}

func (v *EditorView) onDeviceIDChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	n, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
	if err != nil {
		return
	}
	dev.ID = uint32(n)
	v.doc.MarkDirty()
	v.deviceList.Refresh()
	v.revalidate()
}

func (v *EditorView) onDeviceNameChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	dev.Name = text
	v.doc.MarkDirty()
	v.deviceList.Refresh()
	v.revalidate()
}

func (v *EditorView) onDeviceAddressChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	dev.Address = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onDevicePortChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		dev.Port = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return
	}
	dev.Port = uint16(n)
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onDeviceVendorIDChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		dev.VendorID = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return
	}
	dev.VendorID = uint16(n)
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onDeviceVendorNameChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	dev.VendorName = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onDeviceModelNameChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	dev.ModelName = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onDeviceProtocolRevisionChanged(text string) {
	dev := v.currentDevice()
	if dev == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		dev.ProtocolRevision = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 8)
	if err != nil {
		return
	}
	dev.ProtocolRevision = uint8(n)
	v.doc.MarkDirty()
	v.revalidate()
}

// ---- object form ----

func (v *EditorView) rebuildObjectForm() {
	obj := v.currentObject()
	if obj == nil {
		v.objectFormBox.Objects = nil
		v.objInstanceEntry, v.objNameEntry, v.objDescEntry = nil, nil, nil
		v.objPresentValueEntry, v.objPresentValueSelect = nil, nil
		v.objUnitsEntry, v.objWritableCheck, v.objCommandableCheck = nil, nil, nil
		v.objRelinquishEntry, v.objRelinquishSelect = nil, nil
		v.objInitialPriorityEntry, v.objNumberOfStatesEntry, v.objCovIncrementEntry = nil, nil, nil
		v.objectFormBox.Refresh()
		return
	}

	category := classifyObjectType(obj.Type)

	v.objInstanceEntry = widget.NewEntry()
	v.objInstanceEntry.SetText(strconv.FormatUint(uint64(obj.Instance), 10))
	v.objInstanceEntry.OnChanged = v.onObjectInstanceChanged

	v.objNameEntry = widget.NewEntry()
	v.objNameEntry.SetText(obj.Name)
	v.objNameEntry.OnChanged = v.onObjectNameChanged

	v.objDescEntry = widget.NewEntry()
	v.objDescEntry.SetText(obj.Description)
	v.objDescEntry.OnChanged = v.onObjectDescriptionChanged

	presentValueWidget := v.buildValueWidget(category, obj.PresentValue, false)

	v.objUnitsEntry = widget.NewEntry()
	if obj.Units != 0 {
		v.objUnitsEntry.SetText(strconv.FormatUint(uint64(obj.Units), 10))
	}
	v.objUnitsEntry.OnChanged = v.onObjectUnitsChanged

	v.objWritableCheck = widget.NewCheck("", nil)
	v.objWritableCheck.SetChecked(obj.Writable)
	v.objWritableCheck.OnChanged = v.onObjectWritableChanged

	v.objCommandableCheck = widget.NewCheck("", nil)
	v.objCommandableCheck.SetChecked(obj.Commandable)
	v.objCommandableCheck.OnChanged = v.onObjectCommandableChanged

	relinquishWidget := v.buildValueWidget(category, obj.RelinquishDefault, true)

	v.objInitialPriorityEntry = widget.NewEntry()
	if obj.InitialPriority != 0 {
		v.objInitialPriorityEntry.SetText(strconv.FormatUint(uint64(obj.InitialPriority), 10))
	}
	v.objInitialPriorityEntry.OnChanged = v.onObjectInitialPriorityChanged

	v.objCovIncrementEntry = widget.NewEntry()
	v.objCovIncrementEntry.SetText(strconv.FormatFloat(obj.COVIncrement, 'g', -1, 64))
	v.objCovIncrementEntry.OnChanged = v.onObjectCOVIncrementChanged

	items := []*widget.FormItem{
		widget.NewFormItem("Instance", v.objInstanceEntry),
		widget.NewFormItem("Name", v.objNameEntry),
		widget.NewFormItem("Description", v.objDescEntry),
		widget.NewFormItem("Present Value", presentValueWidget),
		widget.NewFormItem("Units", v.objUnitsEntry),
		widget.NewFormItem("Writable", v.objWritableCheck),
		widget.NewFormItem("Commandable", v.objCommandableCheck),
		widget.NewFormItem("Relinquish Default", relinquishWidget),
		widget.NewFormItem("Initial Priority", v.objInitialPriorityEntry),
	}
	if category == "multistate" {
		v.objNumberOfStatesEntry = widget.NewEntry()
		v.objNumberOfStatesEntry.SetText(strconv.FormatUint(uint64(obj.NumberOfStates), 10))
		v.objNumberOfStatesEntry.OnChanged = v.onObjectNumberOfStatesChanged
		items = append(items, widget.NewFormItem("Number Of States", v.objNumberOfStatesEntry))
	} else {
		v.objNumberOfStatesEntry = nil
	}
	items = append(items, widget.NewFormItem("COV Increment", v.objCovIncrementEntry))

	form := widget.NewForm(items...)
	v.objectFormBox.Objects = []fyne.CanvasObject{form}
	v.objectFormBox.Refresh()

	v.applyCommandableEnablement()
}

// buildValueWidget returns the value editor for category (a float Entry for
// "analog", a widget.Select for "binary", a uint Entry for "multistate"),
// seeded from value, and stores it on the matching present-value/
// relinquish-default field pair depending on isRelinquish.
func (v *EditorView) buildValueWidget(category string, value interface{}, isRelinquish bool) fyne.CanvasObject {
	if category == "binary" {
		sel := widget.NewSelect(binaryPresentValueOptions, nil)
		sel.SetSelected(binaryValueText(value))
		if isRelinquish {
			v.objRelinquishSelect = sel
			v.objRelinquishEntry = nil
			sel.OnChanged = v.onObjectRelinquishSelectChanged
		} else {
			v.objPresentValueSelect = sel
			v.objPresentValueEntry = nil
			sel.OnChanged = v.onObjectPresentValueSelectChanged
		}
		return sel
	}

	entry := widget.NewEntry()
	entry.SetText(valueEntryText(category, value))
	if isRelinquish {
		v.objRelinquishEntry = entry
		v.objRelinquishSelect = nil
		entry.OnChanged = v.onObjectRelinquishEntryChanged
	} else {
		v.objPresentValueEntry = entry
		v.objPresentValueSelect = nil
		entry.OnChanged = v.onObjectPresentValueEntryChanged
	}
	return entry
}

// applyCommandableEnablement enables the Relinquish Default and Initial
// Priority editors only when the current object is Commandable.
func (v *EditorView) applyCommandableEnablement() {
	obj := v.currentObject()
	commandable := obj != nil && obj.Commandable

	setDisableable := func(w interface {
		Enable()
		Disable()
	}, enabled bool) {
		if enabled {
			w.Enable()
		} else {
			w.Disable()
		}
	}

	if v.objRelinquishEntry != nil {
		setDisableable(v.objRelinquishEntry, commandable)
	}
	if v.objRelinquishSelect != nil {
		setDisableable(v.objRelinquishSelect, commandable)
	}
	if v.objInitialPriorityEntry != nil {
		setDisableable(v.objInitialPriorityEntry, commandable)
	}
}

func (v *EditorView) onObjectInstanceChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	n, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
	if err != nil {
		return
	}
	obj.Instance = uint32(n)
	v.doc.MarkDirty()
	v.objectList.Refresh()
	v.revalidate()
}

func (v *EditorView) onObjectNameChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.Name = text
	v.doc.MarkDirty()
	v.objectList.Refresh()
	v.revalidate()
}

func (v *EditorView) onObjectDescriptionChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.Description = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectUnitsChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		obj.Units = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return
	}
	obj.Units = uint32(n)
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectWritableChanged(checked bool) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.Writable = checked
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectCommandableChanged(checked bool) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.Commandable = checked
	if checked {
		obj.Writable = true
		setCheckSilently(v.objWritableCheck, true)
		v.objWritableCheck.Disable()
	} else {
		v.objWritableCheck.Enable()
	}
	v.applyCommandableEnablement()
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectInitialPriorityChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		obj.InitialPriority = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 8)
	if err != nil {
		return
	}
	obj.InitialPriority = uint8(n)
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectNumberOfStatesChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		obj.NumberOfStates = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return
	}
	obj.NumberOfStates = uint32(n)
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectCOVIncrementChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		obj.COVIncrement = 0
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return
	}
	obj.COVIncrement = n
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectPresentValueEntryChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	value, ok := parseCategoryValue(classifyObjectType(obj.Type), text)
	if !ok {
		return
	}
	obj.PresentValue = value
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectPresentValueSelectChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.PresentValue = text
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectRelinquishEntryChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	if strings.TrimSpace(text) == "" {
		obj.RelinquishDefault = nil
		v.doc.MarkDirty()
		v.revalidate()
		return
	}
	value, ok := parseCategoryValue(classifyObjectType(obj.Type), text)
	if !ok {
		return
	}
	obj.RelinquishDefault = value
	v.doc.MarkDirty()
	v.revalidate()
}

func (v *EditorView) onObjectRelinquishSelectChanged(text string) {
	obj := v.currentObject()
	if obj == nil {
		return
	}
	obj.RelinquishDefault = text
	v.doc.MarkDirty()
	v.revalidate()
}

// parseCategoryValue parses text into the Go value a scenario present_value
// / relinquish_default field expects for category ("analog" -> float64,
// "multistate" -> uint32); ok is false if text does not parse or category
// is not one of those two (binary values are committed straight from the
// select widget's text, never through this function).
func parseCategoryValue(category, text string) (interface{}, bool) {
	text = strings.TrimSpace(text)
	switch category {
	case "analog":
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, false
		}
		return n, true
	case "multistate":
		n, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return nil, false
		}
		return uint32(n), true
	default:
		return nil, false
	}
}

// valueEntryText renders value (whatever numeric Go type YAML/JSON decoding
// or a previous edit left it as) as the text an analog/multistate value
// Entry should display; "" for nil or an unrecognized type.
func valueEntryText(category string, value interface{}) string {
	switch category {
	case "analog":
		f, ok := toFloat64(value)
		if !ok {
			return ""
		}
		return strconv.FormatFloat(f, 'g', -1, 64)
	case "multistate":
		n, ok := toUint32Value(value)
		if !ok {
			return ""
		}
		return strconv.FormatUint(uint64(n), 10)
	default:
		return ""
	}
}

// binaryValueText renders value as the binaryPresentValueOptions entry it
// corresponds to, defaulting to "inactive" (mirroring the simulator's own
// nil/zero default).
func binaryValueText(value interface{}) string {
	switch v := value.(type) {
	case string:
		switch strings.ToLower(v) {
		case "active", "true", "1":
			return "active"
		default:
			return "inactive"
		}
	case bool:
		if v {
			return "active"
		}
		return "inactive"
	case uint32:
		if v == 1 {
			return "active"
		}
		return "inactive"
	case int:
		if v == 1 {
			return "active"
		}
		return "inactive"
	default:
		return "inactive"
	}
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

func toUint32Value(value interface{}) (uint32, bool) {
	switch v := value.(type) {
	case uint32:
		return v, true
	case uint64:
		return uint32(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	default:
		return 0, false
	}
}

// ---- validation + title ----

// revalidate recomputes both the client-side field errors and the
// authoritative Document.Validate() result, then refreshes every widget
// that displays them.
func (v *EditorView) revalidate() {
	v.fieldErrors = scenariodoc.FieldErrors(v.doc.Scenario())
	err := v.doc.Validate()
	if err != nil {
		v.summaryLabel.SetText(err.Error())
		v.saveBtn.Disable()
	} else {
		v.summaryLabel.SetText("valid")
		v.saveBtn.Enable()
	}
	v.refreshFieldHints()
	v.refreshTitle()
}

// refreshFieldHints applies the current fieldErrors to whichever
// device/object entries are on screen.
func (v *EditorView) refreshFieldHints() {
	if v.selectedDevice >= 0 {
		prefix := fmt.Sprintf("devices[%d]", v.selectedDevice)
		setEntryHint(v.devIDEntry, v.fieldErrors[prefix+".id"])
		setEntryHint(v.devNameEntry, v.fieldErrors[prefix+".name"])
		setEntryHint(v.devAddressEntry, v.fieldErrors[prefix+".address"])
		setEntryHint(v.devPortEntry, v.fieldErrors[prefix+".port"])
	}
	if v.selectedDevice >= 0 && v.selectedObject >= 0 {
		prefix := fmt.Sprintf("devices[%d].objects[%d]", v.selectedDevice, v.selectedObject)
		setEntryHint(v.objNameEntry, v.fieldErrors[prefix+".name"])
		setEntryHint(v.objInstanceEntry, v.fieldErrors[prefix+".instance"])
		setEntryHint(v.objInitialPriorityEntry, v.fieldErrors[prefix+".initial_priority"])
		setEntryHint(v.objCovIncrementEntry, v.fieldErrors[prefix+".cov_increment"])
		setEntryHint(v.objNumberOfStatesEntry, v.fieldErrors[prefix+".number_of_states"])
		setEntryHint(v.objPresentValueEntry, v.fieldErrors[prefix+".present_value"])
		setEntryHint(v.objRelinquishEntry, v.fieldErrors[prefix+".relinquish_default"])
	}
}

// refreshTitle updates the title label to the document's path (or
// "untitled" if it has never been saved), suffixed with " *" while dirty.
func (v *EditorView) refreshTitle() {
	path := v.doc.Path()
	if path == "" {
		path = "untitled"
	}
	if v.doc.Dirty() {
		path += " *"
	}
	v.titleLabel.SetText(path)
}

// ---- toolbar actions ----

func (v *EditorView) onNew() {
	v.newDocument()
}

// newDocument replaces the held document with a fresh scenariodoc.New()
// and refreshes every view that depends on it.
func (v *EditorView) newDocument() {
	v.doc = scenariodoc.New()
	v.selectedDevice = -1
	v.selectedObject = -1
	v.deviceList.Refresh()
	v.refreshNetworkForm()
	if len(v.doc.Scenario().Devices) > 0 {
		v.selectDevice(0)
	} else {
		v.rebuildDeviceForm()
		v.rebuildObjectForm()
	}
	v.revalidate()
}

// openPath loads the scenario at path, replacing the held document on
// success. Exported to the test seam only (unexported); the toolbar's Open
// button drives it via dialog.ShowFileOpen.
func (v *EditorView) openPath(path string) error {
	doc, err := scenariodoc.Load(path)
	if err != nil {
		return err
	}
	v.doc = doc
	v.selectedDevice = -1
	v.selectedObject = -1
	v.deviceList.Refresh()
	v.refreshNetworkForm()
	if len(v.doc.Scenario().Devices) > 0 {
		v.selectDevice(0)
	} else {
		v.rebuildDeviceForm()
		v.rebuildObjectForm()
	}
	v.revalidate()
	return nil
}

// save writes the document to its current path. Returns scenariodoc.ErrNoPath
// if it has never been saved.
func (v *EditorView) save() error {
	err := v.doc.Save()
	v.revalidate()
	return err
}

// saveAs writes the document to path and, on success, makes it the
// document's new destination.
func (v *EditorView) saveAs(path string) error {
	err := v.doc.SaveAs(path)
	v.revalidate()
	return err
}

func (v *EditorView) onOpen() {
	win := currentWindow()
	if win == nil {
		return
	}
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		if loadErr := v.openPath(path); loadErr != nil {
			v.shell.SetStatus("open failed: " + loadErr.Error())
		}
	}, win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml", ".json"}))
	fd.Show()
}

func (v *EditorView) onSave() {
	if v.doc.Path() == "" {
		v.onSaveAs()
		return
	}
	if err := v.save(); err != nil {
		v.shell.SetStatus("save failed: " + err.Error())
	}
}

func (v *EditorView) onSaveAs() {
	win := currentWindow()
	if win == nil {
		return
	}
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		path := writer.URI().Path()
		writer.Close()
		if saveErr := v.saveAs(path); saveErr != nil {
			v.shell.SetStatus("save failed: " + saveErr.Error())
		}
	}, win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml", ".json"}))
	fd.Show()
}

// currentWindow returns the application's (single, Wave-1) window, or nil
// if none exists yet — the same resolution browser.go's write dialog uses.
func currentWindow() fyne.Window {
	windows := fyne.CurrentApp().Driver().AllWindows()
	if len(windows) == 0 {
		return nil
	}
	return windows[0]
}

// ---- small widget helpers ----

func setEntrySilently(e *widget.Entry, text string) {
	if e == nil {
		return
	}
	prev := e.OnChanged
	e.OnChanged = nil
	e.SetText(text)
	e.OnChanged = prev
}

func setSelectSilently(s *widget.Select, text string) {
	if s == nil {
		return
	}
	prev := s.OnChanged
	s.OnChanged = nil
	s.SetSelected(text)
	s.OnChanged = prev
}

func setCheckSilently(c *widget.Check, checked bool) {
	if c == nil {
		return
	}
	prev := c.OnChanged
	c.OnChanged = nil
	c.SetChecked(checked)
	c.OnChanged = prev
}

func setEntryHint(e *widget.Entry, msg string) {
	if e == nil {
		return
	}
	e.AlwaysShowValidationError = true
	if msg == "" {
		e.SetValidationError(nil)
		return
	}
	e.SetValidationError(errors.New(msg))
}
