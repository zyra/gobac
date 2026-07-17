package bacnet

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// ReadAccessSpecProperty identifies a single property, and optionally an
// array index within it, to read from an object via ReadPropertyMultiple.
type ReadAccessSpecProperty struct {
	ID       types.PropertyId
	Index    uint32 // valid only when HasIndex
	HasIndex bool
}

// ReadAccessSpec names one object and the properties to read from it.
type ReadAccessSpec struct {
	Object     types.ObjectId
	Properties []ReadAccessSpecProperty
}

// ReadPropertyMultiple issues a confirmed ReadPropertyMultiple (service
// choice 14) request to address and returns one result per requested
// object, preserving request order.
func (s *Server) ReadPropertyMultiple(ctx context.Context, address net.IP, specs []ReadAccessSpec) ([]*pdu.ReadAccessResult, error) {
	if address == nil || address.Equal(net.IP{0, 0, 0, 0}) {
		return nil, errors.New("received a nil or empty device IP")
	}
	if len(specs) == 0 {
		return nil, errors.New("received no read access specifications")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pduSpecs := make([]pdu.ReadAccessSpec, len(specs))
	for i := range specs {
		object := specs[i].Object
		properties := make([]pdu.PropertyReference, len(specs[i].Properties))
		for j, property := range specs[i].Properties {
			properties[j] = pdu.PropertyReference{
				ID:       property.ID,
				Index:    property.Index,
				HasIndex: property.HasIndex,
			}
		}
		pduSpecs[i] = pdu.ReadAccessSpec{
			ObjectId:   &object,
			Properties: properties,
		}
	}

	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceReadPropertyMultiple, &pdu.ReadPropertyMultiplePdu{
		Specs: pduSpecs,
	}, address)

	if err := req.Send(address, s); err != nil {
		return nil, err
	}

	timer := time.NewTimer(s.DefaultTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case <-timer.C:
		return nil, errors.New("timeout")

	case data := <-req.Data():
		defer data.Release()
		if data.Successful() {
			if val, ok := data.ResponseData().(*pdu.ReadPropertyMultipleAck); ok {
				results := make([]*pdu.ReadAccessResult, len(val.Results))
				for i := range val.Results {
					results[i] = &val.Results[i]
				}
				return results, nil
			} else {
				return nil, errors.New("could not parse response")
			}
		} else if data.Errored() {
			return nil, errors.New(data.ErrorMessage())
		} else if data.Aborted() {
			return nil, errors.New(data.AbortReason())
		} else if data.Rejected() {
			return nil, errors.New(data.RejectReason())
		} else {
			return nil, errors.New("internal error")
		}
	}
}
