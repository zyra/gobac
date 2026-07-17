package scenariodoc

import (
	"fmt"

	"github.com/zyra/gobac/v2/simulator"
)

// AddDevice appends a new device with the next free id starting at 1 (the
// smallest positive id not already in use) and name "device-N", marks the
// document dirty, and returns a pointer into the live Devices slice so
// callers can fill in further fields.
func (d *Document) AddDevice() *simulator.DeviceSpec {
	id := nextFreeID(deviceIDs(d.scenario.Devices))
	d.scenario.Devices = append(d.scenario.Devices, simulator.DeviceSpec{
		ID:   id,
		Name: fmt.Sprintf("device-%d", id),
	})
	d.MarkDirty()
	return &d.scenario.Devices[len(d.scenario.Devices)-1]
}

// RemoveDevice removes the device at index i. Out-of-range indices are a
// no-op (no dirty mark).
func (d *Document) RemoveDevice(i int) {
	if i < 0 || i >= len(d.scenario.Devices) {
		return
	}
	d.scenario.Devices = append(d.scenario.Devices[:i], d.scenario.Devices[i+1:]...)
	d.MarkDirty()
}

// AddObject appends a new object of objType (any of the 9 scenario type
// names, hyphenated or not — see canonicalObjectType) to device devIdx, at
// the next free instance starting at 1 and named "<type>-N". For
// multi-state types it also seeds number_of_states: 2 and present_value: 1
// so the fresh object validates on its own. Returns nil if devIdx is
// out of range or objType is not a supported scenario object type (no
// mutation, no dirty mark, in either case).
func (d *Document) AddObject(devIdx int, objType string) *simulator.ObjectSpec {
	if devIdx < 0 || devIdx >= len(d.scenario.Devices) {
		return nil
	}
	canon, ok := canonicalObjectType(objType)
	if !ok {
		return nil
	}
	dev := &d.scenario.Devices[devIdx]
	instance := nextFreeID(objectInstances(dev.Objects, canon))
	spec := simulator.ObjectSpec{
		Type:     canon,
		Instance: instance,
		Name:     fmt.Sprintf("%s-%d", canon, instance),
	}
	if isMultiStateType(canon) {
		spec.NumberOfStates = 2
		spec.PresentValue = uint32(1)
	}
	dev.Objects = append(dev.Objects, spec)
	d.MarkDirty()
	return &dev.Objects[len(dev.Objects)-1]
}

// RemoveObject removes the object at index objIdx on device devIdx. Either
// index out of range is a no-op (no dirty mark).
func (d *Document) RemoveObject(devIdx, objIdx int) {
	if devIdx < 0 || devIdx >= len(d.scenario.Devices) {
		return
	}
	dev := &d.scenario.Devices[devIdx]
	if objIdx < 0 || objIdx >= len(dev.Objects) {
		return
	}
	dev.Objects = append(dev.Objects[:objIdx], dev.Objects[objIdx+1:]...)
	d.MarkDirty()
}

// deviceIDs collects the set of ids already in use.
func deviceIDs(devices []simulator.DeviceSpec) map[uint32]struct{} {
	ids := make(map[uint32]struct{}, len(devices))
	for _, dev := range devices {
		ids[dev.ID] = struct{}{}
	}
	return ids
}

// objectInstances collects the set of instances already in use among
// objects of the given canonical type on one device.
func objectInstances(objects []simulator.ObjectSpec, canonType string) map[uint32]struct{} {
	instances := make(map[uint32]struct{}, len(objects))
	for _, obj := range objects {
		if canon, ok := canonicalObjectType(obj.Type); ok && canon == canonType {
			instances[obj.Instance] = struct{}{}
		}
	}
	return instances
}

// nextFreeID returns the smallest id >= 1 not present in used.
func nextFreeID(used map[uint32]struct{}) uint32 {
	for id := uint32(1); ; id++ {
		if _, taken := used[id]; !taken {
			return id
		}
	}
}
