package store

import (
	"sort"
	"sync"
	"time"
)

// DeviceKey uniquely identifies a device row: its BACnet device instance
// plus the IP it was last seen from.
type DeviceKey struct {
	Instance uint32
	IP       string
}

// DeviceRow is one row of the discovery table.
type DeviceRow struct {
	Key          DeviceKey
	Port         uint16
	VendorID     uint32
	MaxApdu      uint32
	Segmentation uint8
	// Source records where this row came from: "network" for a real
	// Who-Is sighting, "local-sim" for a device injected by the
	// in-process simulator quickstart (G8).
	Source   string
	LastSeen time.Time
}

// DeviceStore is a thread-safe, observable cache of discovered devices.
type DeviceStore struct {
	mu   sync.RWMutex
	rows map[DeviceKey]DeviceRow

	listeners *listenerSet

	// Now returns the current time; Upsert uses it to stamp a row's
	// LastSeen. Exported so tests can inject a fixed clock. Defaults to
	// time.Now and is safe to reassign only before concurrent use begins.
	Now func() time.Time
}

// NewDeviceStore creates an empty DeviceStore.
func NewDeviceStore() *DeviceStore {
	return &DeviceStore{
		rows:      make(map[DeviceKey]DeviceRow),
		listeners: newListenerSet(),
		Now:       time.Now,
	}
}

// Upsert inserts or merges row into the store, stamping LastSeen from
// Now(), and notifies listeners.
//
// A row already recorded with Source "local-sim" keeps that Source even
// when re-sighted with Source "network" — a quickstart device stays
// identified as local-sim even if it also answers a real Who-Is sweep on
// loopback. Every other field is replaced with the incoming row's value.
func (s *DeviceStore) Upsert(row DeviceRow) {
	s.mu.Lock()
	if existing, ok := s.rows[row.Key]; ok && existing.Source == "local-sim" && row.Source == "network" {
		row.Source = "local-sim"
	}
	if s.Now != nil {
		row.LastSeen = s.Now()
	} else {
		row.LastSeen = time.Now()
	}
	s.rows[row.Key] = row
	s.mu.Unlock()

	s.listeners.notify()
}

// Remove deletes key from the store, if present, and notifies listeners.
func (s *DeviceStore) Remove(key DeviceKey) {
	s.mu.Lock()
	delete(s.rows, key)
	s.mu.Unlock()

	s.listeners.notify()
}

// Clear empties the store and notifies listeners.
func (s *DeviceStore) Clear() {
	s.mu.Lock()
	s.rows = make(map[DeviceKey]DeviceRow)
	s.mu.Unlock()

	s.listeners.notify()
}

// Snapshot returns an independent copy of all rows sorted by Instance
// ascending, then IP ascending. Mutating the returned slice does not
// affect the store.
func (s *DeviceStore) Snapshot() []DeviceRow {
	s.mu.RLock()
	rows := make([]DeviceRow, 0, len(s.rows))
	for _, row := range s.rows {
		rows = append(rows, row)
	}
	s.mu.RUnlock()

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Key.Instance != rows[j].Key.Instance {
			return rows[i].Key.Instance < rows[j].Key.Instance
		}
		return rows[i].Key.IP < rows[j].Key.IP
	})
	return rows
}

// Len returns the number of rows currently stored.
func (s *DeviceStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rows)
}

// AddListener registers fn to be called (synchronously, on the goroutine
// that performed the mutation, outside the store's lock) after every
// Upsert, Remove, or Clear. It returns a function that unregisters fn.
func (s *DeviceStore) AddListener(fn func()) (remove func()) {
	return s.listeners.add(fn)
}
