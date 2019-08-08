package bacnet

import (
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"time"
)

type CovNotifier struct {
	out            chan *pdu.CovNotification
	err            chan error
	subscriptionId uint8
	cancel         bool
	handler        chan *Request
}

func (n *CovNotifier) Data() <-chan *pdu.CovNotification {
	return n.out
}

func (n *CovNotifier) Error() <-chan error {
	return n.err
}

func (s *Server) SubscribeCov(deviceIP *net.IP,
	objectType types.ObjectType,
	objectInstance types.Uint16,
	processID uint8,
	cancel bool) (*CovNotifier, error) {
	req := NewRequest()
	defer req.Release()

	handler := make(chan *Request, 128)

	s.SetCovHandler(processID, handler)

	req.SetConfirmedService(types.ConfirmedServiceSubscribeCov, &pdu.SubscribeCov{
		ObjectId: &types.ObjectId{
			Instance: objectInstance,
			Type:     objectType,
		},
		ProcessIdentifier: processID,
	})

	if err := req.Send(deviceIP, s); err != nil {
		return nil, err
	}

	// The first response will be our Ack or Abort/Error/Reject
	// Let's listen to that first and figure out if this request was successful;
	// and if it is, we can start listening for the next Acks and emit the covData.
	select {
	case <-time.After(s.DefaultTimeout):
		return nil, errors.New("timeout")

	case data := <-req.Data():
		if data.Successful() {
			if !cancel {
				n := &CovNotifier{
					out:            make(chan *pdu.CovNotification, 128),
					err:            make(chan error, 128),
					subscriptionId: processID,
					handler:        handler,
				}

				go n.startListening(s)

				return n, nil
			}

			return nil, nil
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

func (n *CovNotifier) startListening(server *Server) {
	defer server.RemoveCovHandler(n.subscriptionId)
	for {
		if data := <-n.handler; data.Successful() {
			if val, ok := data.ResponseData().(*pdu.CovNotification); ok {
				n.out <- val
			} else {
				n.err <- errors.New("error decoding response")
			}
		} else if data.Errored() {
			n.err <- errors.New(data.ErrorMessage())
		} else if data.Aborted() {
			n.err <- errors.New(data.AbortReason())
		} else if data.Rejected() {
			n.err <- errors.New(data.RejectReason())
		} else {
			n.err <- errors.New("internal error")
		}
	}
}
