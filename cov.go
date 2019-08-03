package gobac

import (
	"errors"
	"fmt"
)

type CovRequest struct {
	ReadPropertyRequest
	out            chan *Property
	err            chan error
	subscriptionId uint32
}

func (r *CovRequest) Data() <-chan *Property {
	return r.out
}

func (r *CovRequest) Error() <-chan error {
	return r.err
}

func (s *Server) SendCovRequest(object *Object, subscriptionId uint32) (*CovRequest, error) {
	req := &CovRequest{
		out:            make(chan *Property),
		err:            make(chan error),
		subscriptionId: subscriptionId,
		ReadPropertyRequest: ReadPropertyRequest{
			ConfirmedRequest: s.NewConfirmedRequest(),
			object:           object,
			propertyId:       PropPresentValue,
		},
	}

	req.Target = object.Device.IPAddress

	if e := req.EncodeCovSubscribeApdu(); e != nil {
		return nil, e
	}

	if req.Len() >= int(object.Device.MaxAPDU) {
		return nil, errors.New("apdu too large")
	}

	if err := req.Send(); err != nil {
		return nil, err
	}

	// The first response will be our Ack or Abort/Error/Reject
	// Let's listen to that first and figure out if this request was successful;
	// and if it is, we can start listening for the next Acks and emit the data.
	data := <-req.ConfirmedRequest.Data()

	if data.Failed {
		return nil, errors.New("device responded with abort, error, or reject")
	}

	go req.startListening()

	return req, nil
}

func (r *CovRequest) startListening() {
	defer r.Cleanup()
	for {
		data := <-r.ConfirmedRequest.Data()
		if data.Failed {
			r.done <- struct{}{}
			r.err <- errors.New("device rejected request")
			break
		} else if data.PduType == PduTypeSimpleAck {
			// dismiss
			fmt.Println("simple ack!")
		} else {
			go r.emitData(data)
		}
		continue
	}
}

func (r *CovRequest) emitData(data *Response) {
	prop := Property{}

	err := data.DecodeReadPropertyApdu(r.object, r.propertyId, &prop)

	if err != nil {
		r.err <- err
	} else {
		r.out <- &prop
	}
}

func (r *CovRequest) EncodeCovSubscribeApdu() (err error) {
	err = r.AppendByte(PduTypeConfirmedServiceRequest)
	err = r.AppendByte(5)
	err = r.AppendByte(r.InvokeID)
	err = r.AppendByte(ConfirmedServiceSubscribeCov)

	err = r.EncodeTag(0, true, getUnsignedLen(uint(r.subscriptionId)))
	err = r.EncodeUnsigned(r.subscriptionId) // subscription ID

	// monitored object
	err = r.EncodeTag(1, true, 4)
	err = r.EncodeObjectId(r.object.Type, r.object.Instance)

	err = r.EncodeTag(2, true, 1)
	err = r.AppendByte(1)

	err = r.EncodeTag(3, true, 4)
	err = r.EncodeUnsigned32(0)
	return err
}
