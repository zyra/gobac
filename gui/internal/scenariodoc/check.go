package scenariodoc

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/zyra/gobac/v2/simulator"
)

// objectTypeAliases mirrors simulator.objectTypeNumber's name table (which
// is unexported): both the hyphenated and non-hyphenated multi-state
// spellings the simulator accepts map to the same canonical, hyphenated
// name used as the object's "type" field and as this package's map keys.
var objectTypeAliases = map[string]string{
	"analog-input":       "analog-input",
	"analog-output":      "analog-output",
	"analog-value":       "analog-value",
	"binary-input":       "binary-input",
	"binary-output":      "binary-output",
	"binary-value":       "binary-value",
	"multistate-input":   "multi-state-input",
	"multistate-output":  "multi-state-output",
	"multistate-value":   "multi-state-value",
	"multi-state-input":  "multi-state-input",
	"multi-state-output": "multi-state-output",
	"multi-state-value":  "multi-state-value",
}

// canonicalObjectType normalizes a scenario object type name the same way
// simulator.objectTypeNumber does (lowercased, "_" folded to "-") and looks
// it up in objectTypeAliases.
func canonicalObjectType(name string) (string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", "-"))
	canon, ok := objectTypeAliases[normalized]
	return canon, ok
}

// isMultiStateType reports whether canon (as returned by
// canonicalObjectType) is one of the three multi-state object types.
func isMultiStateType(canon string) bool {
	switch canon {
	case "multi-state-input", "multi-state-output", "multi-state-value":
		return true
	default:
		return false
	}
}

// FieldErrors is a client-side mirror of simulator.Scenario.Validate's
// cross-field rules (simulator/scenario.go), keyed by field path (e.g.
// "devices[0].objects[1].initial_priority") so a form UI can show one
// message per offending field instead of only the first error
// Scenario.Validate would return. On any behavioral conflict between this
// function and simulator.Scenario.Validate, Validate is authoritative —
// this exists only to give immediate, per-field feedback in the editor.
func FieldErrors(s *simulator.Scenario) map[string]string {
	errs := make(map[string]string)

	if s.Version != simulator.ScenarioVersion {
		errs["version"] = fmt.Sprintf("scenario version %d is not supported", s.Version)
	}

	switch s.Network.Mode {
	case "single-device", "multi-ip", "multi-port":
	default:
		errs["network.mode"] = fmt.Sprintf("network mode %q is not supported", s.Network.Mode)
	}

	if len(s.Devices) == 0 {
		errs["devices"] = "at least one device is required"
	} else if s.Network.Mode == "single-device" && len(s.Devices) != 1 {
		errs["devices"] = "single-device mode requires exactly one device"
	}

	deviceIDs := make(map[uint32]struct{}, len(s.Devices))
	endpoints := make(map[string]struct{}, len(s.Devices))

	for i := range s.Devices {
		dev := &s.Devices[i]
		devPrefix := fmt.Sprintf("devices[%d]", i)

		if dev.ID > simulator.MaxObjectInstance {
			errs[devPrefix+".id"] = fmt.Sprintf("device %d exceeds BACnet object-instance range", dev.ID)
		}
		if _, exists := deviceIDs[dev.ID]; exists {
			errs[devPrefix+".id"] = fmt.Sprintf("duplicate device id %d", dev.ID)
		} else {
			deviceIDs[dev.ID] = struct{}{}
		}

		if strings.TrimSpace(dev.Name) == "" {
			errs[devPrefix+".name"] = fmt.Sprintf("device %d name is required", dev.ID)
		}

		if s.Network.Mode == "multi-ip" && strings.TrimSpace(dev.Address) == "" {
			errs[devPrefix+".address"] = fmt.Sprintf("device %d address is required in multi-ip mode", dev.ID)
		}

		if s.Network.Mode != "single-device" {
			endpoint := dev.Address + ":" + strconv.Itoa(int(dev.Port))
			if _, exists := endpoints[endpoint]; exists {
				errs[devPrefix+".port"] = fmt.Sprintf("duplicate device endpoint %s", endpoint)
			} else {
				endpoints[endpoint] = struct{}{}
			}
		}

		// The simulator's own Validate additionally seeds this set with the
		// device's own {device, dev.ID} object id, since a user-defined
		// object could collide with it. That never happens here: this
		// package's canonicalObjectType only recognizes the 9 editable
		// scenario object types (G7), never "device", so no such seed is
		// needed to mirror the check.
		objectKeys := make(map[string]struct{}, len(dev.Objects))
		for j := range dev.Objects {
			obj := &dev.Objects[j]
			objPrefix := fmt.Sprintf("%s.objects[%d]", devPrefix, j)

			canon, ok := canonicalObjectType(obj.Type)
			if !ok {
				errs[objPrefix+".type"] = fmt.Sprintf("object type %q is not supported", obj.Type)
				continue
			}

			if obj.Instance > simulator.MaxObjectInstance {
				errs[objPrefix+".instance"] = fmt.Sprintf("device %d object %s instance is out of range", dev.ID, obj.Name)
			}
			key := canon + ":" + strconv.FormatUint(uint64(obj.Instance), 10)
			if _, exists := objectKeys[key]; exists {
				errs[objPrefix+".instance"] = fmt.Sprintf("device %d has duplicate object %s %d", dev.ID, canon, obj.Instance)
			} else {
				objectKeys[key] = struct{}{}
			}

			if strings.TrimSpace(obj.Name) == "" {
				errs[objPrefix+".name"] = fmt.Sprintf("device %d object %d name is required", dev.ID, obj.Instance)
			}

			// commandable => writable is an auto-implication the simulator
			// applies silently (Scenario.Validate mutates Writable rather
			// than failing) — not a validation error, so nothing is added
			// here; the editor UI (G7) reflects the implication directly
			// in the Writable checkbox instead.

			if obj.COVIncrement < 0 || math.IsNaN(obj.COVIncrement) || math.IsInf(obj.COVIncrement, 0) {
				errs[objPrefix+".cov_increment"] = fmt.Sprintf("device %d object %s has an invalid cov_increment", dev.ID, obj.Name)
			}

			if obj.InitialPriority != 0 {
				switch {
				case !obj.Commandable || obj.PresentValue == nil:
					errs[objPrefix+".initial_priority"] = fmt.Sprintf(
						"device %d object %s initial_priority requires commandable and present_value", dev.ID, obj.Name)
				case obj.InitialPriority > simulator.PrioritySlots || obj.InitialPriority == 6:
					errs[objPrefix+".initial_priority"] = fmt.Sprintf(
						"device %d object %s has an invalid initial_priority (%d)", dev.ID, obj.Name, obj.InitialPriority)
				}
			}

			if isMultiStateType(canon) {
				if obj.NumberOfStates == 0 {
					errs[objPrefix+".number_of_states"] = fmt.Sprintf(
						"device %d object %s number_of_states must be greater than zero", dev.ID, obj.Name)
				} else {
					if n, ok := toUint32(obj.PresentValue); ok && n > obj.NumberOfStates {
						errs[objPrefix+".present_value"] = fmt.Sprintf(
							"device %d object %s present_value exceeds number_of_states", dev.ID, obj.Name)
					}
					if obj.Commandable && obj.RelinquishDefault != nil {
						if n, ok := toUint32(obj.RelinquishDefault); ok && n > obj.NumberOfStates {
							errs[objPrefix+".relinquish_default"] = fmt.Sprintf(
								"device %d object %s relinquish_default exceeds number_of_states", dev.ID, obj.Name)
						}
					}
				}
			}
		}
	}

	return errs
}

// toUint32 mirrors simulator's numericUint32 for the value shapes YAML/JSON
// decoding (and this package's own mutators) can produce. ok is false for
// nil or a value that isn't a non-negative whole number.
func toUint32(value interface{}) (uint32, bool) {
	switch v := value.(type) {
	case uint32:
		return v, true
	case uint64:
		if v > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case int:
		if v < 0 || uint64(v) > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 || uint64(v) > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case float64:
		if v < 0 || v > math.MaxUint32 || v != math.Trunc(v) {
			return 0, false
		}
		return uint32(v), true
	default:
		return 0, false
	}
}
