package bacnet

import (
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"time"
)

func (s *Server) WriteProperty(deviceAddress *net.IP,
	objectType,
	objectInstance types.Uint16,
	propertyId types.PropertyId,
	tag types.DataType,
	priority uint8,
	value interface{}) error {

	if deviceAddress == nil || deviceAddress.Equal(net.IP{0, 0, 0, 0}) {
		return errors.New("received a nil or empty device IP")
	}

	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceWriteProperty, &pdu.WriteProperty{
		Property: &types.Property{
			ObjectId: &types.ObjectId{
				Type:     objectType,
				Instance: objectInstance,
			},
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

	tc := time.After(s.DefaultTimeout)

	select {
	case <-tc:
		return errors.New("timeout")

	case data := <-req.Data():
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
