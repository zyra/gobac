package bacnet

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// WriteAccessSpecProperty is one BACnetPropertyValue: the property (and
// optional array index) to write, the value to write to it, and an optional
// per-write priority, as used by WritePropertyMultiple.
type WriteAccessSpecProperty struct {
	ID          types.PropertyId
	Index       uint32 // valid only when HasIndex
	HasIndex    bool
	Tag         types.DataType
	Value       interface{}
	Priority    uint8 // valid only when HasPriority; 1..16
	HasPriority bool
}

// WriteAccessSpec names one object and the property values to write to it.
type WriteAccessSpec struct {
	Object     types.ObjectId
	Properties []WriteAccessSpecProperty
}

// WritePropertyMultipleError describes why a WritePropertyMultiple request
// failed, and which property was the first one that could not be written.
type WritePropertyMultipleError struct {
	ErrorClass          types.ErrorClass
	ErrorCode           types.ErrorCode
	FirstFailedObjectId *types.ObjectId
	FirstFailedProperty types.PropertyId
	FirstFailedIndex    uint32
	FirstFailedHasIndex bool
}

func (e *WritePropertyMultipleError) Error() string {
	return e.ErrorClass.String() + " " + e.ErrorCode.String()
}

// WritePropertyMultiple issues a confirmed WritePropertyMultiple (service
// choice 16) request to address. Every write is applied atomically: if any
// property fails validation, none of them are written. On failure the
// returned error is a *WritePropertyMultipleError when the device follows the
// standard WritePropertyMultiple-Error production.
func (s *Server) WritePropertyMultiple(ctx context.Context, address net.IP, specs []WriteAccessSpec) error {
	if address == nil || address.Equal(net.IP{0, 0, 0, 0}) {
		return errors.New("received a nil or empty device IP")
	}
	if len(specs) == 0 {
		return errors.New("received no write access specifications")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pduSpecs := make([]pdu.WriteAccessSpec, len(specs))
	for i := range specs {
		object := specs[i].Object
		properties := make([]pdu.PropertyWrite, len(specs[i].Properties))
		for j, property := range specs[i].Properties {
			properties[j] = pdu.PropertyWrite{
				ID:          property.ID,
				Index:       property.Index,
				HasIndex:    property.HasIndex,
				Values:      []*types.PropertyValue{{Type: property.Tag, Value: property.Value}},
				Priority:    property.Priority,
				HasPriority: property.HasPriority,
			}
		}
		pduSpecs[i] = pdu.WriteAccessSpec{
			ObjectId:   &object,
			Properties: properties,
		}
	}

	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceWritePropertyMultiple, &pdu.WritePropertyMultiplePdu{
		Specs: pduSpecs,
	}, address)

	if err := req.Send(address, s); err != nil {
		return err
	}

	timer := time.NewTimer(s.DefaultTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return errors.New("timeout")

	case data := <-req.Data():
		defer data.Release()
		if data.Successful() {
			return nil
		} else if data.Errored() {
			if wpmErr, ok := data.ResponseData().(*pdu.WritePropertyMultipleError); ok {
				return &WritePropertyMultipleError{
					ErrorClass:          wpmErr.Class,
					ErrorCode:           wpmErr.Code,
					FirstFailedObjectId: wpmErr.FirstFailed.ObjectId,
					FirstFailedProperty: wpmErr.FirstFailed.ID,
					FirstFailedIndex:    wpmErr.FirstFailed.Index,
					FirstFailedHasIndex: wpmErr.FirstFailed.HasIndex,
				}
			}
			return errors.New(data.ErrorMessage())
		} else if data.Aborted() {
			return errors.New(data.AbortReason())
		} else if data.Rejected() {
			return errors.New(data.RejectReason())
		} else {
			return errors.New("internal error")
		}
	}
}
