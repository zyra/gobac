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

// validateWrite runs every check Write performs, without mutating the
// property, and returns the priority that apply should use. Callers that
// need to validate a batch of writes before applying any of them (e.g.
// WritePropertyMultiple) should call validateWrite for each write and only
// call apply once every write in the batch has validated successfully.
func (p *Property) validateWrite(values []Value, priority uint8) (uint8, error) {
	if !p.Writable {
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

type Object struct {
	ID         ObjectID
	Name       string
	Properties map[uint32]*Property
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
		if propertyID == uint32(types.PropertyPriorityArray) {
			if pv := object.Properties[uint32(types.PropertyPresentValue)]; pv != nil && pv.RelinquishDefault != nil {
				return pv.priorityArrayProperty().Read(arrayIndex)
			}
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
		if propertyID == uint32(types.PropertyPriorityArray) {
			if pv := object.Properties[uint32(types.PropertyPresentValue)]; pv != nil && pv.RelinquishDefault != nil {
				return ErrWriteDenied
			}
		}
		return ErrUnknownProperty
	}
	_, err := property.validateWrite(values, priority)
	return err
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
		if propertyID == uint32(types.PropertyPriorityArray) {
			if pv := object.Properties[uint32(types.PropertyPresentValue)]; pv != nil && pv.RelinquishDefault != nil {
				return ErrWriteDenied
			}
		}
		return ErrUnknownProperty
	}
	return property.Write(values, priority)
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
