package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// maxLegacyInstance is the largest object instance the 16-bit legacy client
// API can address. Above this, requests are rejected until library task L2
// (22-bit ReadObjectProperty/WriteObjectProperty) merges.
const maxLegacyInstance = 65535

// startTimeout bounds how long Start waits for the underlying server to
// finish binding its sockets.
const startTimeout = 5 * time.Second

var errSessionNotStarted = errors.New("session not started")

// Live is the production Session implementation, backed by a real
// bacnet.Server sending/receiving over UDP.
type Live struct {
	mu  sync.Mutex
	srv *bacnet.Server
}

// NewLive creates a Live session. It does nothing network-visible until
// Start is called.
func NewLive() *Live {
	return &Live{}
}

// Start implements Session.
func (l *Live) Start(cfg Config) error {
	l.mu.Lock()
	if l.srv != nil {
		l.mu.Unlock()
		return ErrAlreadyStarted
	}
	l.mu.Unlock()

	serverConfig := bacnet.NewServerConfig()
	serverConfig.SetInterfaceName(cfg.Interface)
	serverConfig.SetListenPort(cfg.Port)
	serverConfig.SetServerBBMDPort(cfg.LocalPort)
	serverConfig.SetReceiveErrors(true)

	srv, err := bacnet.NewServer(serverConfig)
	if err != nil {
		return err
	}

	go srv.Listen(context.Background())

	select {
	case <-srv.Start():
	case <-time.After(startTimeout):
		srv.Shutdown()
		return errors.New("bacnet server did not start")
	}

	l.mu.Lock()
	l.srv = srv
	l.mu.Unlock()
	return nil
}

// Stop implements Session. It is idempotent: calling it when the session
// isn't started is a no-op.
func (l *Live) Stop() error {
	l.mu.Lock()
	srv := l.srv
	l.srv = nil
	l.mu.Unlock()

	if srv == nil {
		return nil
	}

	srv.Shutdown()
	<-srv.Close()
	return nil
}

func (l *Live) server() *bacnet.Server {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.srv
}

// Discover implements Session.
func (l *Live) Discover(ctx context.Context, timeout time.Duration) (<-chan DeviceSummary, error) {
	srv := l.server()
	if srv == nil {
		return nil, errSessionNotStarted
	}

	devices, err := srv.WhoIs(ctx, timeout)
	if err != nil {
		return nil, err
	}

	out := make(chan DeviceSummary)
	go func() {
		defer close(out)
		for dev := range devices {
			summary := DeviceSummary{
				Instance:     dev.ObjectId.InstanceNumber(),
				IP:           dev.IPAddress,
				VendorID:     uint32(dev.VendorID),
				MaxApdu:      uint32(dev.MaxAPDU),
				Segmentation: uint32(dev.Segmentation),
			}
			select {
			case out <- summary:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// ReadProperty implements Session.
func (l *Live) ReadProperty(ctx context.Context, dev Address, obj ObjectRef, prop uint32) ([]Value, error) {
	if obj.Instance > maxLegacyInstance {
		return nil, fmt.Errorf("object instance %d requires library 22-bit support (pending L2)", obj.Instance)
	}

	srv := l.server()
	if srv == nil {
		return nil, errSessionNotStarted
	}

	// TODO(L2): swap to srv.ReadObjectProperty (22-bit instance) once
	// library task L2 merges; this is otherwise a one-line change.
	values, err := srv.ReadProperty(ctx, dev.IP, types.Uint16(obj.Type), types.Uint16(obj.Instance), prop)
	if err != nil {
		return nil, err
	}

	result := make([]Value, len(values))
	for i, v := range values {
		result[i] = toValue(v)
	}
	return result, nil
}

// ReadMultiple implements Session. Until library task L1
// (ReadPropertyMultiple) merges, it issues one ReadProperty call per
// property and collects per-property failures instead of failing the whole
// batch.
func (l *Live) ReadMultiple(ctx context.Context, dev Address, specs []ReadSpec) ([]ObjectResult, error) {
	// TODO(L1): swap to srv.ReadPropertyMultiple once library task L1
	// merges; the ObjectResult shape is stable either way.
	results := make([]ObjectResult, len(specs))
	for i, spec := range specs {
		result := ObjectResult{Object: spec.Object}
		for _, prop := range spec.Properties {
			values, err := l.ReadProperty(ctx, dev, spec.Object, prop)
			if err != nil {
				if result.Errors == nil {
					result.Errors = make(map[uint32]error)
				}
				result.Errors[prop] = err
				continue
			}
			result.Values = append(result.Values, values...)
		}
		results[i] = result
	}
	return results, nil
}

// Write implements Session, writing an object's Present_Value.
func (l *Live) Write(ctx context.Context, dev Address, obj ObjectRef, w WriteRequest) error {
	if obj.Instance > maxLegacyInstance {
		return fmt.Errorf("object instance %d requires library 22-bit support (pending L2)", obj.Instance)
	}

	srv := l.server()
	if srv == nil {
		return errSessionNotStarted
	}

	// TODO(L2): swap to srv.WriteObjectProperty (22-bit instance) once
	// library task L2 merges; this is otherwise a one-line change.
	return srv.WriteProperty(ctx, dev.IP, types.Uint16(obj.Type), types.Uint16(obj.Instance),
		types.PropertyId(types.PropertyPresentValue), w.Tag, w.Priority, w.Value)
}

// toValue converts a decoded wire property value into the facade Value
// type, unwrapping library newtypes (types.Real, types.Double, ...) into
// their plain Go equivalents so callers never need to import
// bacnet/types.
func toValue(v *types.PropertyValue) Value {
	value := v.Value
	switch typed := value.(type) {
	case types.Real:
		value = float32(typed)
	case types.Double:
		value = float64(typed)
	case types.CharacterString:
		value = typed.Value
	}
	return Value{Tag: v.Type, Value: value}
}
