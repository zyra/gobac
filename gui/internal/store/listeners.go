// Package store holds thread-safe, observable caches (device discovery
// results, per-device object/property data) that the UI layer binds to. It
// has no dependency on Fyne or sockets; it is fed by the session layer and
// read by the ui layer (see the layering rules in gui-architecture.md §6).
package store

import "sync"

// listenerSet manages a set of removable change-notification callbacks,
// shared by DeviceStore and ObjectCache. Callbacks are invoked
// synchronously by notify's caller, outside of any store data lock, so a
// listener is free to call back into the store (including add/remove)
// without deadlocking.
type listenerSet struct {
	mu        sync.Mutex
	listeners map[int]func()
	nextID    int
}

// newListenerSet creates an empty listenerSet.
func newListenerSet() *listenerSet {
	return &listenerSet{listeners: make(map[int]func())}
}

// add registers fn and returns a function that unregisters it. remove is
// safe to call more than once; subsequent calls are no-ops.
func (l *listenerSet) add(fn func()) (remove func()) {
	l.mu.Lock()
	id := l.nextID
	l.nextID++
	l.listeners[id] = fn
	l.mu.Unlock()

	return func() {
		l.mu.Lock()
		delete(l.listeners, id)
		l.mu.Unlock()
	}
}

// notify invokes every currently registered listener exactly once. It
// takes a snapshot of the listener set under lock, then calls each
// function after releasing the lock.
func (l *listenerSet) notify() {
	l.mu.Lock()
	fns := make([]func(), 0, len(l.listeners))
	for _, fn := range l.listeners {
		fns = append(fns, fn)
	}
	l.mu.Unlock()

	for _, fn := range fns {
		fn()
	}
}
