package bacnet

import (
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"time"
)

func (s *Server) ReadProperty(address *net.IP, objectType, objectInstance types.Uint16, propertyId types.PropertyId) ([]*types.PropertyValue, error) {
	req := NewRequest()
	defer req.Release()

	req.SetConfirmedService(types.ConfirmedServiceReadProperty, &pdu.ReadPropertyPdu{
		Property: &types.Property{
			ObjectId: &types.ObjectId{
				Type:     objectType,
				Instance: objectInstance,
			},
			ID: propertyId,
		},
	})

	if err := req.Send(address, s); err != nil {
		return nil, err
	}

	tc := time.After(s.DefaultTimeout)

	select {
	case <-tc:
		return nil, errors.New("timeout")

	case data := <-req.Data():
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
