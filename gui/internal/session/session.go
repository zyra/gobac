// Package session wraps the github.com/zyra/gobac/v2 client stack (the
// "bacnet.Server" request API) behind a small facade so the rest of the GUI
// never imports library wire types directly. It has no dependency on Fyne
// and must remain unit-testable on its own (see the global constraints in
// gui-architecture.md §6.3).
package session

import (
	"context"
	"errors"
	"net"
	"time"
)

// ErrAlreadyStarted is returned by Start when the session is already
// listening.
var ErrAlreadyStarted = errors.New("session already started")

// Config configures a Session's underlying BACnet/IP listener.
type Config struct {
	// Interface is the OS network interface name to listen on (e.g. "eno0",
	// "lo"). Not an IP address.
	Interface string
	// Port is the BACnet/IP port devices are reached on. It is also the
	// port this session's own listener binds to: the wrapped library always
	// sends requests to the port it is itself listening on, mirroring
	// real-world BACnet/IP networks where every participant shares one
	// well-known port.
	Port uint16
	// LocalPort is the port this session identifies itself with (the
	// library's "server BBMD port"). It does not affect where outgoing
	// requests are sent.
	LocalPort uint16
}

// Address identifies a BACnet/IP device to talk to.
type Address struct {
	IP net.IP
}

// ObjectRef identifies a BACnet object. Instance is carried as a full
// 22-bit value (up to 4194303); the current implementation can only reach
// instances up to 65535 until library task L2 lands (see ReadProperty).
type ObjectRef struct {
	Type     uint16
	Instance uint32
}

// DeviceSummary is the discovery-facing view of a BACnet device, derived
// from an I-Am response.
type DeviceSummary struct {
	Instance uint32
	IP       net.IP
	// Port is the UDP port the device is reachable at. The library's
	// client stack always sends requests to the same port it is itself
	// listening on (Config.Port) — real-world BACnet/IP participants
	// share one well-known port — so this is the session's own
	// configured port, not a value decoded from the I-Am response.
	Port         uint16
	VendorID     uint32
	MaxApdu      uint32
	Segmentation uint32
}

// Value is a single decoded property value: an application tag (matching
// github.com/zyra/gobac/v2/bacnet/types data-type constants, e.g. 4 for
// Real) plus its native Go value.
type Value struct {
	Tag   uint8
	Value interface{}
}

// WriteRequest describes a write to an object's Present_Value. Wave-1 scope
// only supports writing Present_Value (the common commandable-object case);
// there is no property selector.
type WriteRequest struct {
	Tag      uint8
	Priority uint8
	Value    interface{}
}

// ReadSpec batches multiple property reads against one object.
type ReadSpec struct {
	Object     ObjectRef
	Properties []uint32
}

// ObjectResult is the outcome of a batched read against one object: values
// that succeeded, plus a per-property-id map of errors for the ones that
// didn't. A batch call itself only fails for transport-level problems; a
// per-property error does not fail the whole call.
type ObjectResult struct {
	Object ObjectRef
	Values []Value
	Errors map[uint32]error
}

// Session is the facade the rest of the GUI uses to talk BACnet/IP. Live is
// the production implementation; other packages depend only on this
// interface so they can be tested with fakes.
type Session interface {
	// Start configures and starts the underlying BACnet/IP listener,
	// blocking until it is ready to send/receive. Calling Start twice
	// without an intervening Stop returns ErrAlreadyStarted.
	Start(cfg Config) error
	// Stop shuts the listener down. It is safe to call multiple times.
	Stop() error
	// Discover broadcasts a Who-Is and streams I-Am responses until timeout
	// elapses or ctx is done, then closes the returned channel.
	Discover(ctx context.Context, timeout time.Duration) (<-chan DeviceSummary, error)
	// ReadProperty reads a single property from a single object.
	ReadProperty(ctx context.Context, dev Address, obj ObjectRef, prop uint32) ([]Value, error)
	// ReadMultiple reads properties from one or more objects, collecting
	// per-property errors into each ObjectResult rather than failing the
	// whole batch.
	ReadMultiple(ctx context.Context, dev Address, specs []ReadSpec) ([]ObjectResult, error)
	// Write writes an object's Present_Value.
	Write(ctx context.Context, dev Address, obj ObjectRef, w WriteRequest) error
}
