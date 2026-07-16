package simulator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/zyra/gobac/bacnet/types"
	"gopkg.in/yaml.v2"
)

const ScenarioVersion = 1

type Scenario struct {
	Version int           `json:"version" yaml:"version"`
	Seed    int64         `json:"seed,omitempty" yaml:"seed,omitempty"`
	Network NetworkConfig `json:"network" yaml:"network"`
	Devices []DeviceSpec  `json:"devices" yaml:"devices"`
}

type NetworkConfig struct {
	Mode      string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`
	Port      uint16 `json:"port,omitempty" yaml:"port,omitempty"`
}

type DeviceSpec struct {
	ID               uint32       `json:"id" yaml:"id"`
	Address          string       `json:"address,omitempty" yaml:"address,omitempty"`
	Port             uint16       `json:"port,omitempty" yaml:"port,omitempty"`
	Name             string       `json:"name" yaml:"name"`
	VendorID         uint16       `json:"vendor_id,omitempty" yaml:"vendor_id,omitempty"`
	VendorName       string       `json:"vendor_name,omitempty" yaml:"vendor_name,omitempty"`
	ModelName        string       `json:"model_name,omitempty" yaml:"model_name,omitempty"`
	ProtocolRevision uint8        `json:"protocol_revision,omitempty" yaml:"protocol_revision,omitempty"`
	Objects          []ObjectSpec `json:"objects,omitempty" yaml:"objects,omitempty"`
}

type ObjectSpec struct {
	Type              string      `json:"type" yaml:"type"`
	Instance          uint32      `json:"instance" yaml:"instance"`
	Name              string      `json:"name" yaml:"name"`
	Description       string      `json:"description,omitempty" yaml:"description,omitempty"`
	PresentValue      interface{} `json:"present_value,omitempty" yaml:"present_value,omitempty"`
	Units             uint32      `json:"units,omitempty" yaml:"units,omitempty"`
	Writable          bool        `json:"writable,omitempty" yaml:"writable,omitempty"`
	Commandable       bool        `json:"commandable,omitempty" yaml:"commandable,omitempty"`
	RelinquishDefault interface{} `json:"relinquish_default,omitempty" yaml:"relinquish_default,omitempty"`
	NumberOfStates    uint32      `json:"number_of_states,omitempty" yaml:"number_of_states,omitempty"`
	COVIncrement      float64     `json:"cov_increment,omitempty" yaml:"cov_increment,omitempty"`
}

func DecodeScenario(reader io.Reader, format string) (*Scenario, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	scenario := &Scenario{}
	switch strings.ToLower(format) {
	case "json":
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(scenario); err != nil {
			return nil, err
		}
	case "yaml", "yml", "":
		if err := yaml.UnmarshalStrict(data, scenario); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported scenario format %q", format)
	}

	applyScenarioDefaults(scenario)
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	return scenario, nil
}

func (s *Scenario) Validate() error {
	if s.Version != ScenarioVersion {
		return fmt.Errorf("scenario version %d is not supported", s.Version)
	}
	switch s.Network.Mode {
	case "single-device", "multi-ip", "multi-port":
	default:
		return fmt.Errorf("network mode %q is not supported", s.Network.Mode)
	}
	if len(s.Devices) == 0 {
		return errors.New("at least one device is required")
	}
	if s.Network.Mode == "single-device" && len(s.Devices) != 1 {
		return errors.New("single-device mode requires exactly one device")
	}

	deviceIDs := make(map[uint32]struct{}, len(s.Devices))
	endpoints := make(map[string]struct{}, len(s.Devices))
	for i := range s.Devices {
		device := &s.Devices[i]
		if device.ID > MaxObjectInstance {
			return fmt.Errorf("device %d exceeds BACnet object-instance range", device.ID)
		}
		if _, exists := deviceIDs[device.ID]; exists {
			return fmt.Errorf("duplicate device id %d", device.ID)
		}
		deviceIDs[device.ID] = struct{}{}
		if strings.TrimSpace(device.Name) == "" {
			return fmt.Errorf("device %d name is required", device.ID)
		}
		if s.Network.Mode == "multi-ip" && strings.TrimSpace(device.Address) == "" {
			return fmt.Errorf("device %d address is required in multi-ip mode", device.ID)
		}
		endpoint := device.Address + ":" + strconv.Itoa(int(device.Port))
		if s.Network.Mode != "single-device" {
			if _, exists := endpoints[endpoint]; exists {
				return fmt.Errorf("duplicate device endpoint %s", endpoint)
			}
			endpoints[endpoint] = struct{}{}
		}

		objectIDs := map[ObjectID]struct{}{{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}: {}}
		for j := range device.Objects {
			object := &device.Objects[j]
			objectType, err := objectTypeNumber(object.Type)
			if err != nil {
				return fmt.Errorf("device %d: %v", device.ID, err)
			}
			if object.Instance > MaxObjectInstance {
				return fmt.Errorf("device %d object %s instance is out of range", device.ID, object.Name)
			}
			id := ObjectID{Type: objectType, Instance: object.Instance}
			if _, exists := objectIDs[id]; exists {
				return fmt.Errorf("device %d has duplicate object %s", device.ID, id)
			}
			objectIDs[id] = struct{}{}
			if strings.TrimSpace(object.Name) == "" {
				return fmt.Errorf("device %d object %s name is required", device.ID, id)
			}
			if object.Commandable && !object.Writable {
				object.Writable = true
			}
		}
	}
	return nil
}

func (s *Scenario) BuildNetwork() (*Network, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	network := NewNetwork()
	for _, spec := range s.Devices {
		device, err := buildDevice(spec)
		if err != nil {
			return nil, err
		}
		if err := network.AddDevice(device); err != nil {
			return nil, err
		}
	}
	return network, nil
}

func applyScenarioDefaults(scenario *Scenario) {
	if scenario.Version == 0 {
		scenario.Version = ScenarioVersion
	}
	if scenario.Network.Mode == "" {
		if len(scenario.Devices) == 1 {
			scenario.Network.Mode = "single-device"
		} else {
			scenario.Network.Mode = "multi-ip"
		}
	}
	if scenario.Network.Port == 0 {
		scenario.Network.Port = 0xbac0
	}
	for i := range scenario.Devices {
		device := &scenario.Devices[i]
		if device.Port == 0 {
			device.Port = scenario.Network.Port
		}
		if device.VendorName == "" {
			device.VendorName = "GoBAC"
		}
		if device.ModelName == "" {
			device.ModelName = "GoBAC Simulator"
		}
		if device.ProtocolRevision == 0 {
			// Revision 14 predates the mandatory Network Port object and matches
			// the initial simulator's implemented object profile.
			device.ProtocolRevision = 14
		}
	}
}

func buildDevice(spec DeviceSpec) (*Device, error) {
	device := &Device{
		ID:        spec.ID,
		Name:      spec.Name,
		Address:   spec.Address,
		Port:      spec.Port,
		VendorID:  spec.VendorID,
		ModelName: spec.ModelName,
		Objects:   make(map[ObjectID]*Object, len(spec.Objects)+1),
	}

	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: spec.ID}
	deviceObject := &Object{ID: deviceID, Name: spec.Name, Properties: make(map[uint32]*Property)}
	addCommonProperties(deviceObject, spec.Name, "")
	deviceObject.Properties[types.PropertyVendorIdentifier] = scalarProperty(types.PropertyVendorIdentifier, types.TagUnsigned, uint32(spec.VendorID), false)
	deviceObject.Properties[types.PropertyVendorName] = scalarProperty(types.PropertyVendorName, types.TagCharacterString, spec.VendorName, false)
	deviceObject.Properties[types.PropertyModelName] = scalarProperty(types.PropertyModelName, types.TagCharacterString, spec.ModelName, false)
	deviceObject.Properties[types.PropertyMaxApduLengthAccepted] = scalarProperty(types.PropertyMaxApduLengthAccepted, types.TagUnsigned, uint32(types.MaxApdu), false)
	deviceObject.Properties[types.PropertySegmentationSupported] = scalarProperty(types.PropertySegmentationSupported, types.TagEnumerated, uint32(3), false)
	deviceObject.Properties[types.PropertyProtocolVersion] = scalarProperty(types.PropertyProtocolVersion, types.TagUnsigned, uint32(1), false)
	deviceObject.Properties[types.PropertyProtocolRevision] = scalarProperty(types.PropertyProtocolRevision, types.TagUnsigned, uint32(spec.ProtocolRevision), false)
	device.Objects[deviceID] = deviceObject

	objectList := []Value{{Tag: types.TagObjectId, Value: deviceID}}
	for _, objectSpec := range spec.Objects {
		object, err := buildObject(objectSpec)
		if err != nil {
			return nil, fmt.Errorf("device %d: %v", spec.ID, err)
		}
		device.Objects[object.ID] = object
		objectList = append(objectList, Value{Tag: types.TagObjectId, Value: object.ID})
	}
	deviceObject.Properties[types.PropertyObjectList] = &Property{ID: types.PropertyObjectList, Array: true, Values: objectList}
	return device, nil
}

func buildObject(spec ObjectSpec) (*Object, error) {
	objectType, err := objectTypeNumber(spec.Type)
	if err != nil {
		return nil, err
	}
	object := &Object{
		ID:         ObjectID{Type: objectType, Instance: spec.Instance},
		Name:       spec.Name,
		Properties: make(map[uint32]*Property),
	}
	addCommonProperties(object, spec.Name, spec.Description)

	tag, normalized, err := normalizePresentValue(objectType, spec.PresentValue)
	if err != nil {
		return nil, err
	}
	property := scalarProperty(types.PropertyPresentValue, tag, normalized, spec.Writable)
	property.COVIncrement = spec.COVIncrement
	if spec.Commandable {
		defaultTag, defaultValue, err := normalizePresentValue(objectType, spec.RelinquishDefault)
		if err != nil {
			return nil, err
		}
		if spec.RelinquishDefault == nil {
			defaultTag, defaultValue = tag, normalized
		}
		property.RelinquishDefault = &Value{Tag: defaultTag, Value: defaultValue}
	}
	object.Properties[types.PropertyPresentValue] = property

	if spec.Units != 0 {
		object.Properties[types.PropertyUnits] = scalarProperty(types.PropertyUnits, types.TagEnumerated, spec.Units, false)
	}
	if spec.NumberOfStates != 0 {
		object.Properties[types.PropertyNumberOfStates] = scalarProperty(types.PropertyNumberOfStates, types.TagUnsigned, spec.NumberOfStates, false)
	}
	return object, nil
}

func addCommonProperties(object *Object, name, description string) {
	object.Properties[types.PropertyObjectIdentifier] = scalarProperty(types.PropertyObjectIdentifier, types.TagObjectId, object.ID, false)
	object.Properties[types.PropertyObjectName] = scalarProperty(types.PropertyObjectName, types.TagCharacterString, name, false)
	object.Properties[types.PropertyObjectType] = scalarProperty(types.PropertyObjectType, types.TagEnumerated, uint32(object.ID.Type), false)
	if description != "" {
		object.Properties[types.PropertyDescription] = scalarProperty(types.PropertyDescription, types.TagCharacterString, description, false)
	}
}

func scalarProperty(id uint32, tag uint8, value interface{}, writable bool) *Property {
	return &Property{ID: id, Writable: writable, Values: []Value{{Tag: tag, Value: value}}}
}

func objectTypeNumber(name string) (uint16, error) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", "-"))
	typesByName := map[string]uint16{
		"analog-input":       uint16(types.ObjectTypeAnalogInput),
		"analog-output":      uint16(types.ObjectTypeAnalogOutput),
		"analog-value":       uint16(types.ObjectTypeAnalogValue),
		"binary-input":       uint16(types.ObjectTypeBinaryInput),
		"binary-output":      uint16(types.ObjectTypeBinaryOutput),
		"binary-value":       uint16(types.ObjectTypeBinaryValue),
		"multistate-input":   uint16(types.ObjectTypeMultiStateInput),
		"multistate-output":  uint16(types.ObjectTypeMultiStateOutput),
		"multistate-value":   uint16(types.ObjectTypeMultiStateValue),
		"multi-state-input":  uint16(types.ObjectTypeMultiStateInput),
		"multi-state-output": uint16(types.ObjectTypeMultiStateOutput),
		"multi-state-value":  uint16(types.ObjectTypeMultiStateValue),
	}
	value, ok := typesByName[normalized]
	if !ok {
		return 0, fmt.Errorf("object type %q is not supported", name)
	}
	return value, nil
}

func normalizePresentValue(objectType uint16, value interface{}) (uint8, interface{}, error) {
	switch objectType {
	case uint16(types.ObjectTypeAnalogInput), uint16(types.ObjectTypeAnalogOutput), uint16(types.ObjectTypeAnalogValue):
		if value == nil {
			value = float64(0)
		}
		number, err := numericFloat64(value)
		return types.TagReal, float32(number), err
	case uint16(types.ObjectTypeBinaryInput), uint16(types.ObjectTypeBinaryOutput), uint16(types.ObjectTypeBinaryValue):
		switch typed := value.(type) {
		case nil:
			return types.TagEnumerated, uint32(0), nil
		case bool:
			if typed {
				return types.TagEnumerated, uint32(1), nil
			}
			return types.TagEnumerated, uint32(0), nil
		case string:
			switch strings.ToLower(typed) {
			case "active", "true", "1":
				return types.TagEnumerated, uint32(1), nil
			case "inactive", "false", "0":
				return types.TagEnumerated, uint32(0), nil
			}
		}
		number, err := numericUint32(value)
		if number > 1 && err == nil {
			err = errors.New("binary present value must be 0 or 1")
		}
		return types.TagEnumerated, number, err
	default:
		if value == nil {
			value = uint32(1)
		}
		number, err := numericUint32(value)
		if number == 0 && err == nil {
			err = errors.New("multi-state present value starts at 1")
		}
		return types.TagUnsigned, number, err
	}
}

func numericFloat64(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case uint64:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("%v is not numeric", value)
	}
}

func numericUint32(value interface{}) (uint32, error) {
	switch typed := value.(type) {
	case uint32:
		return typed, nil
	case uint64:
		return uint32(typed), nil
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("%d is negative", typed)
		}
		return uint32(typed), nil
	case int64:
		if typed < 0 {
			return 0, fmt.Errorf("%d is negative", typed)
		}
		return uint32(typed), nil
	case float64:
		if typed < 0 || typed != float64(uint32(typed)) {
			return 0, fmt.Errorf("%v is not an unsigned integer", value)
		}
		return uint32(typed), nil
	default:
		return 0, fmt.Errorf("%v is not an unsigned integer", value)
	}
}
