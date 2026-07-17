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

func (s *Server) WriteProperty(ctx context.Context, deviceAddress net.IP, objectType, objectInstance types.Uint16, propertyId types.PropertyId, tag types.DataType, priority uint8, value interface{}) error {
	return s.writeProperty(ctx, deviceAddress, types.ObjectId{Type: objectType, Instance: objectInstance}, propertyId, tag, priority, value)
}

// WriteObjectProperty is WriteProperty with a full 22-bit object instance.
func (s *Server) WriteObjectProperty(ctx context.Context, deviceAddress net.IP, object types.ObjectId, propertyId types.PropertyId, tag types.DataType, priority uint8, value interface{}) error {
	if object.InstanceNumber() > types.BacnetMaxInstance {
		return fmt.Errorf("object instance %d exceeds maximum of %d", object.InstanceNumber(), types.BacnetMaxInstance)
	}
	if object.Type >= types.BacnetMaxObject+1 {
		return fmt.Errorf("object type %d exceeds maximum of %d", object.Type, types.BacnetMaxObject)
	}
	return s.writeProperty(ctx, deviceAddress, object, propertyId, tag, priority, value)
}

func (s *Server) writeProperty(ctx context.Context, deviceAddress net.IP, object types.ObjectId, propertyId types.PropertyId, tag types.DataType, priority uint8, value interface{}) error {

	if deviceAddress == nil || deviceAddress.Equal(net.IP{0, 0, 0, 0}) {
		return errors.New("received a nil or empty device IP")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceWriteProperty, &pdu.WriteProperty{
		Property: &types.Property{
			ObjectId: &object,
			Values: []*types.PropertyValue{
				{
					Type:  tag,
					Value: value,
				},
			},
			ID: propertyId,
		},
		Priority: priority,
	}, deviceAddress)

	if err := req.Send(deviceAddress, s); err != nil {
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
