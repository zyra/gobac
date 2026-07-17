package store

import (
	"sync"

	"github.com/zyra/gobac/gui/internal/session"
)

// ObjectEntry is one object found in a device's Object_List.
type ObjectEntry struct {
	Type     uint16
	Instance uint32
	Name     string
}

// PropertyEntry is one property read result for an object, as displayed by
// the object browser's property table: either Values is populated or Err
// holds a non-empty message for that property.
type PropertyEntry struct {
	ID     uint32
	Values []session.Value
	Err    string
}

// objectKey identifies one object within one device's cache.
type objectKey struct {
	Device   DeviceKey
	Type     uint16
	Instance uint32
}

// ObjectCache is a thread-safe, observable per-device cache of object
// lists and their property read results.
type ObjectCache struct {
	mu         sync.RWMutex
	objects    map[DeviceKey][]ObjectEntry
	properties map[objectKey][]PropertyEntry

	listeners *listenerSet
}

// NewObjectCache creates an empty ObjectCache.
func NewObjectCache() *ObjectCache {
	return &ObjectCache{
		objects:    make(map[DeviceKey][]ObjectEntry),
		properties: make(map[objectKey][]PropertyEntry),
		listeners:  newListenerSet(),
	}
}

// SetObjects replaces the object list cached for key and notifies
// listeners.
func (c *ObjectCache) SetObjects(key DeviceKey, entries []ObjectEntry) {
	cp := make([]ObjectEntry, len(entries))
	copy(cp, entries)

	c.mu.Lock()
	c.objects[key] = cp
	c.mu.Unlock()

	c.listeners.notify()
}

// Objects returns an independent copy of the object list cached for key
// (nil if none has been set).
func (c *ObjectCache) Objects(key DeviceKey) []ObjectEntry {
	c.mu.RLock()
	entries := c.objects[key]
	c.mu.RUnlock()
	if entries == nil {
		return nil
	}
	cp := make([]ObjectEntry, len(entries))
	copy(cp, entries)
	return cp
}

// SetProperties replaces the cached property read results for obj on
// device key and notifies listeners.
func (c *ObjectCache) SetProperties(key DeviceKey, obj session.ObjectRef, props []PropertyEntry) {
	cp := make([]PropertyEntry, len(props))
	copy(cp, props)

	ok := objectKey{Device: key, Type: obj.Type, Instance: obj.Instance}
	c.mu.Lock()
	c.properties[ok] = cp
	c.mu.Unlock()

	c.listeners.notify()
}

// Properties returns an independent copy of the cached property read
// results for obj on device key (nil if none has been set).
func (c *ObjectCache) Properties(key DeviceKey, obj session.ObjectRef) []PropertyEntry {
	ok := objectKey{Device: key, Type: obj.Type, Instance: obj.Instance}

	c.mu.RLock()
	props := c.properties[ok]
	c.mu.RUnlock()
	if props == nil {
		return nil
	}
	cp := make([]PropertyEntry, len(props))
	copy(cp, props)
	return cp
}

// AddListener registers fn to be called (synchronously, on the goroutine
// that performed the mutation, outside the cache's lock) after every
// SetObjects or SetProperties call. It returns a function that
// unregisters fn.
func (c *ObjectCache) AddListener(fn func()) (remove func()) {
	return c.listeners.add(fn)
}
