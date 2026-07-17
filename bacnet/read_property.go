package bacnet

import (
	"context"
	"errors"
	"fmt"
	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
	"net"
	"time"
)

func (s *Server) ReadProperty(ctx context.Context, address net.IP, objectType, objectInstance types.Uint16, propertyId types.PropertyId) ([]*types.PropertyValue, error) {
	return s.readProperty(ctx, address, types.ObjectId{Type: objectType, Instance: objectInstance}, propertyId)
}

// ReadObjectProperty is ReadProperty with a full 22-bit object instance.
func (s *Server) ReadObjectProperty(ctx context.Context, address net.IP, object types.ObjectId, propertyId types.PropertyId) ([]*types.PropertyValue, error) {
	if object.InstanceNumber() > types.BacnetMaxInstance {
		return nil, fmt.Errorf("object instance %d exceeds maximum of %d", object.InstanceNumber(), types.BacnetMaxInstance)
	}
	if object.Type >= types.BacnetMaxObject+1 {
		return nil, fmt.Errorf("object type %d exceeds maximum of %d", object.Type, types.BacnetMaxObject)
	}
	return s.readProperty(ctx, address, object, propertyId)
}

func (s *Server) readProperty(ctx context.Context, address net.IP, object types.ObjectId, propertyId types.PropertyId) ([]*types.PropertyValue, error) {
	if address == nil || address.Equal(net.IP{0, 0, 0, 0}) {
		return nil, errors.New("received a nil or empty device IP")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceReadProperty, &pdu.ReadPropertyPdu{
		Property: &types.Property{
			ObjectId: &object,
			ID:       propertyId,
		},
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
			if val, ok := data.ResponseData().(*pdu.ReadPropertyPdu); ok {
				return val.Property.Values, nil
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
