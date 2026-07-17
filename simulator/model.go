package simulator

import (
	"errors"
	"fmt"
	"sync"

	"github.com/zyra/gobac/v2/bacnet/types"
)

const (
	MaxObjectInstance = 0x3fffff
	PrioritySlots     = 16
)

var (
	ErrUnknownDevice     = errors.New("unknown device")
	ErrUnknownObject     = errors.New("unknown object")
	ErrUnknownProperty   = errors.New("unknown property")
	ErrWriteDenied       = errors.New("write access denied")
	ErrInvalidPriority   = errors.New("invalid priority")
	ErrReservedPriority  = errors.New("reserved priority")
	ErrValueOutOfRange   = errors.New("value out of range")
	ErrInvalidDataType   = errors.New("invalid data type")
	ErrPropertyNotArray  = errors.New("property is not an array")
	ErrInvalidArrayIndex = errors.New("invalid array index")
)

type ObjectID struct {
	Type     uint16
	Instance uint32
}

func (id ObjectID) String() string {
	return fmt.Sprintf("%d:%d", id.Type, id.Instance)
}

type Value struct {
	Tag   uint8
	Value interface{}
}

type Property struct {
	ID                uint32
	Values            []Value
	Writable          bool
	Array             bool
	COVIncrement      float64
	RelinquishDefault *Value
	MinimumUnsigned   uint32
	MaximumUnsigned   uint32
	Scalar            bool
	ExpectedTag       uint8
	priorities        [PrioritySlots]*Value
}

func (p *Property) Read(arrayIndex *uint32) ([]Value, error) {
	if arrayIndex == nil {
		return cloneValues(p.effectiveValues()), nil
	}

	if !p.Array {
		return nil, ErrPropertyNotArray
	}

	values := p.effectiveValues()
	if *arrayIndex == 0 {
		return []Value{{Tag: 2, Value: uint32(len(values))}}, nil
	}
	if *arrayIndex == ^uint32(0) {
		return cloneValues(values), nil
	}
	if *arrayIndex > uint32(len(values)) {
		return nil, ErrInvalidArrayIndex
	}
	return []Value{cloneValue(values[*arrayIndex-1])}, nil
}

func (p *Property) Write(values []Value, priority uint8) error {
	resolvedPriority, err := p.validateWrite(values, priority)
	if err != nil {
		return err
	}
	p.apply(values, resolvedPriority)
	return nil
}

// writeBypassingWritable performs the same write Write does except it does
// not require the property to be Writable. It is used for Present_Value
// writes on non-commandable objects while Out_Of_Service is true: the
// standard requires those writes to be accepted regardless of the
// property's normal input/output writability (model.go Device.WriteProperty).
func (p *Property) writeBypassingWritable(values []Value, priority uint8) error {
	resolvedPriority, err := p.validateWriteAllowingReadOnly(values, priority, true)
	if err != nil {
		return err
	}
	p.apply(values, resolvedPriority)
	return nil
}

// validateWrite runs every check Write performs, without mutating the
// property, and returns the priority that apply should use. Callers that
// need to validate a batch of writes before applying any of them (e.g.
// WritePropertyMultiple) should call validateWrite for each write and only
// call apply once every write in the batch has validated successfully.
func (p *Property) validateWrite(values []Value, priority uint8) (uint8, error) {
	return p.validateWriteAllowingReadOnly(values, priority, p.Writable)
}

// validateWriteBypassingWritable is the non-mutating counterpart of
// writeBypassingWritable, for callers (WritePropertyMultiple's validation
// pass) that need to check a would-be OOS-bypassed write without applying it.
func (p *Property) validateWriteBypassingWritable(values []Value, priority uint8) (uint8, error) {
	return p.validateWriteAllowingReadOnly(values, priority, true)
}

func (p *Property) validateWriteAllowingReadOnly(values []Value, priority uint8, writable bool) (uint8, error) {
	if !writable {
		return 0, ErrWriteDenied
	}
	if len(values) == 0 {
		return 0, errors.New("property value is required")
	}
	if err := p.validateValues(values); err != nil {
		return 0, err
	}

	if p.RelinquishDefault != nil {
		if priority == 0 {
			priority = PrioritySlots
		}
		if priority < 1 || priority > PrioritySlots {
			return 0, ErrInvalidPriority
		}
		if priority == 6 {
			return 0, ErrReservedPriority
		}
		return priority, nil
	}

	if priority != 0 {
		return 0, ErrInvalidPriority
	}
	return 0, nil
}

// apply performs the mutation validateWrite already approved. priority is
// only meaningful when the property has a RelinquishDefault (a commandable
// property), matching the resolved priority validateWrite returned.
func (p *Property) apply(values []Value, priority uint8) {
	if p.RelinquishDefault != nil {
		if values[0].Tag == 0 || values[0].Value == nil {
			p.priorities[priority-1] = nil
		} else {
			value := cloneValue(values[0])
			p.priorities[priority-1] = &value
		}
		return
	}
	p.Values = cloneValues(values)
}

func (p *Property) validateValues(values []Value) error {
	if p.Scalar {
		if len(values) != 1 {
			return ErrInvalidDataType
		}
		if values[0].Tag == 0 || values[0].Value == nil {
			if p.RelinquishDefault != nil {
				return nil
			}
			return ErrInvalidDataType
		}
		if values[0].Tag != p.ExpectedTag {
			return ErrInvalidDataType
		}
	}
	if p.MaximumUnsigned == 0 || len(values) == 0 || values[0].Tag == 0 || values[0].Value == nil {
		return nil
	}
	value, ok := values[0].Value.(uint32)
	if !ok || value < p.MinimumUnsigned || value > p.MaximumUnsigned {
		return ErrValueOutOfRange
	}
	return nil
}

func (p *Property) PriorityArray() [PrioritySlots]*Value {
	var result [PrioritySlots]*Value
	for i, value := range p.priorities {
		if value != nil {
			copy := cloneValue(*value)
			result[i] = &copy
		}
	}
	return result
}

// priorityArrayProperty exposes the live command priorities as the BACnet
// Priority_Array property: a 16-slot array whose unset slots are NULL.
func (p *Property) priorityArrayProperty() *Property {
	slots := p.PriorityArray()
	values := make([]Value, PrioritySlots)
	for i, slot := range slots {
		if slot == nil {
			values[i] = Value{Tag: types.TagNull}
		} else {
			values[i] = *slot
		}
	}
	return &Property{ID: uint32(types.PropertyPriorityArray), Array: true, Values: values}
}

func (p *Property) effectiveValues() []Value {
	if p.RelinquishDefault == nil {
		return p.Values
	}
	for _, value := range p.priorities {
		if value != nil {
			return []Value{*value}
		}
	}
	return []Value{*p.RelinquishDefault}
}

// synthesizedProperty builds the standard health/status properties that are
// computed from live object state rather than stored in Object.Properties,
// the same way priorityArrayProperty synthesizes Priority_Array. It returns
// nil when propertyID does not name a synthesized property for object's
// current state (e.g. Relinquish_Default on a non-commandable object, or any
// of these on the Device object, which does not carry them in this
// simulator's profile).
func synthesizedProperty(object *Object, propertyID uint32) *Property {
	if pv := object.Properties[uint32(types.PropertyPresentValue)]; pv != nil {
		switch propertyID {
		case uint32(types.PropertyPriorityArray):
			if pv.RelinquishDefault != nil {
				return pv.priorityArrayProperty()
			}
			return nil
		case uint32(types.PropertyRelinquishDefault):
			if pv.RelinquishDefault != nil {
				return relinquishDefaultProperty(pv)
			}
			return nil
		}
	}
	if object.ID.Type == uint16(types.ObjectTypeDevice) {
		return nil
	}
	switch propertyID {
	case uint32(types.PropertyStatusFlags):
		return statusFlagsProperty(object)
	case uint32(types.PropertyEventState):
		return eventStateProperty()
	case uint32(types.PropertyReliability):
		return reliabilityProperty()
	case uint32(types.PropertyOutOfService):
		return outOfServiceProperty(object)
	}
	return nil
}

// statusFlagsProperty exposes the live Out_Of_Service state as the BACnet
// Status_Flags property: a 4-bit BitString {in-alarm, fault, overridden,
// out-of-service}. in-alarm/fault/overridden are always false in this wave;
// out-of-service mirrors object.OutOfService.
func statusFlagsProperty(object *Object) *Property {
	return &Property{
		ID:     uint32(types.PropertyStatusFlags),
		Values: []Value{{Tag: types.TagBitString, Value: statusFlagsBitString(object.OutOfService)}},
	}
}

func statusFlagsBitString(outOfService bool) types.BitString {
	var octet byte
	if outOfService {
		octet |= 1 << 3
	}
	return types.BitString{octet}
}

// eventStateProperty is always Enumerated 0 (normal) in this wave.
func eventStateProperty() *Property {
	return &Property{ID: uint32(types.PropertyEventState), Values: []Value{{Tag: types.TagEnumerated, Value: uint32(0)}}}
}

// reliabilityProperty is always Enumerated 0 (no-fault-detected) in this wave.
func reliabilityProperty() *Property {
	return &Property{ID: uint32(types.PropertyReliability), Values: []Value{{Tag: types.TagEnumerated, Value: uint32(0)}}}
}

// outOfServiceProperty exposes Out_Of_Service as a plain (non-commandable,
// no priority array) writable Boolean property backed by object.OutOfService.
func outOfServiceProperty(object *Object) *Property {
	return &Property{
		ID:          uint32(types.PropertyOutOfService),
		Writable:    true,
		Scalar:      true,
		ExpectedTag: types.TagBoolean,
		Values:      []Value{{Tag: types.TagBoolean, Value: object.OutOfService}},
	}
}

// relinquishDefaultProperty exposes a commandable Present_Value's configured
// relinquish-default as the read-only Relinquish_Default property.
func relinquishDefaultProperty(pv *Property) *Property {
	return &Property{ID: uint32(types.PropertyRelinquishDefault), Values: []Value{*pv.RelinquishDefault}}
}

type Object struct {
	ID           ObjectID
	Name         string
	Properties   map[uint32]*Property
	OutOfService bool
}

type Device struct {
	ID        uint32
	Name      string
	Address   string
	Port      uint16
	VendorID  uint16
	ModelName string
	Objects   map[ObjectID]*Object
	mu        sync.RWMutex
}

func (d *Device) Object(id ObjectID) (*Object, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	object := d.Objects[id]
	if object == nil {
		return nil, ErrUnknownObject
	}
	return object, nil
}

func (d *Device) ReadProperty(id ObjectID, propertyID uint32, arrayIndex *uint32) ([]Value, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	object := d.Objects[id]
	if object == nil {
		return nil, ErrUnknownObject
	}
	property := object.Properties[propertyID]
	if property == nil {
		if synthesized := synthesizedProperty(object, propertyID); synthesized != nil {
			return synthesized.Read(arrayIndex)
		}
		return nil, ErrUnknownProperty
	}
	return property.Read(arrayIndex)
}

// ValidateWrite runs every check WriteProperty performs, without mutating the
// device, so a caller (e.g. WritePropertyMultiple) can validate a batch of
// writes across possibly-multiple objects and only apply them once every
// write in the batch is known to succeed.
func (d *Device) ValidateWrite(id ObjectID, propertyID uint32, values []Value, priority uint8) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	object := d.Objects[id]
	if object == nil {
		return ErrUnknownObject
	}
	property := object.Properties[propertyID]
	if property == nil {
		return validateSynthesizedWrite(object, propertyID, values, priority)
	}
	if propertyID == uint32(types.PropertyPresentValue) && object.OutOfService && property.RelinquishDefault == nil {
		_, err := property.validateWriteBypassingWritable(values, priority)
		return err
	}
	_, err := property.validateWrite(values, priority)
	return err
}

// validateSynthesizedWrite and writeSynthesizedProperty handle writes to
// property IDs that are not stored in Object.Properties (see
// synthesizedProperty). Out_Of_Service (81) is the only writable one; the
// rest are read-only and report ErrWriteDenied when the object qualifies for
// them, or ErrUnknownProperty otherwise (matching the existing Priority_Array
// behavior this generalizes).
func validateSynthesizedWrite(object *Object, propertyID uint32, values []Value, priority uint8) error {
	synthesized := synthesizedProperty(object, propertyID)
	if synthesized == nil {
		return ErrUnknownProperty
	}
	if propertyID != uint32(types.PropertyOutOfService) {
		return ErrWriteDenied
	}
	_, err := synthesized.validateWrite(values, priority)
	return err
}

// SetLocalTime records a TimeSynchronization / UTCTimeSynchronization sync
// onto the device object's Local_Date (56) and Local_Time (57) properties,
// creating them on first sync (buildDevice intentionally leaves them absent
// until then). The simulator has no UTC-offset model, so
// UTCTimeSynchronization values are stored exactly as received, the same as
// a local TimeSynchronization.
func (d *Device) SetLocalTime(date types.Date, clock types.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: d.ID}
	object := d.Objects[deviceID]
	if object == nil {
		return
	}
	object.Properties[uint32(types.PropertyLocalDate)] = scalarProperty(uint32(types.PropertyLocalDate), types.TagDate, date, false)
	object.Properties[uint32(types.PropertyLocalTime)] = scalarProperty(uint32(types.PropertyLocalTime), types.TagTime, clock, false)
}

func (d *Device) WriteProperty(id ObjectID, propertyID uint32, values []Value, priority uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	object := d.Objects[id]
	if object == nil {
		return ErrUnknownObject
	}
	property := object.Properties[propertyID]
	if property == nil {
		return writeSynthesizedProperty(object, propertyID, values, priority)
	}
	if propertyID == uint32(types.PropertyPresentValue) && object.OutOfService && property.RelinquishDefault == nil {
		return property.writeBypassingWritable(values, priority)
	}
	return property.Write(values, priority)
}

func writeSynthesizedProperty(object *Object, propertyID uint32, values []Value, priority uint8) error {
	synthesized := synthesizedProperty(object, propertyID)
	if synthesized == nil {
		return ErrUnknownProperty
	}
	if propertyID != uint32(types.PropertyOutOfService) {
		return ErrWriteDenied
	}
	if _, err := synthesized.validateWrite(values, priority); err != nil {
		return err
	}
	value, _ := values[0].Value.(bool)
	object.OutOfService = value
	return nil
}

type Network struct {
	Devices map[uint32]*Device
	mu      sync.RWMutex
}

func NewNetwork() *Network {
	return &Network{Devices: make(map[uint32]*Device)}
}

func (n *Network) AddDevice(device *Device) error {
	if device == nil {
		return errors.New("device is required")
	}
	if device.ID > MaxObjectInstance {
		return fmt.Errorf("device id %d exceeds BACnet object-instance range", device.ID)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.Devices[device.ID]; exists {
		return fmt.Errorf("duplicate device id %d", device.ID)
	}
	if device.Objects == nil {
		device.Objects = make(map[ObjectID]*Object)
	}
	n.Devices[device.ID] = device
	return nil
}

func (n *Network) Device(id uint32) (*Device, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	device := n.Devices[id]
	if device == nil {
		return nil, ErrUnknownDevice
	}
	return device, nil
}

func cloneValues(values []Value) []Value {
	result := make([]Value, len(values))
	for i := range values {
		result[i] = cloneValue(values[i])
	}
	return result
}

func cloneValue(value Value) Value {
	if bytes, ok := value.Value.([]byte); ok {
		copy := append([]byte(nil), bytes...)
		value.Value = copy
	}
	return value
}
