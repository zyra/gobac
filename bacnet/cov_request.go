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

type CovNotifier struct {
	out            chan *pdu.CovNotification
	err            chan error
	subscriptionId uint32
	cancel         bool
	handler        chan *Request
	deviceIP       net.IP
}

func (n *CovNotifier) Data() <-chan *pdu.CovNotification {
	return n.out
}

func (n *CovNotifier) Error() <-chan error {
	return n.err
}

func (s *Server) SubscribeCov(ctx context.Context, deviceIP net.IP, objectType types.ObjectType, objectInstance types.Uint16, processID uint8, cancel bool) (*CovNotifier, error) {
	return s.SubscribeCovWithProcessID(ctx, deviceIP, objectType, objectInstance, uint32(processID), cancel)
}

// SubscribeCovWithProcessID subscribes using the complete BACnet Unsigned32
// subscriber process identifier.
func (s *Server) SubscribeCovWithProcessID(ctx context.Context, deviceIP net.IP, objectType types.ObjectType, objectInstance types.Uint16, processID uint32, cancel bool) (*CovNotifier, error) {
	return s.subscribeCov(ctx, deviceIP, types.ObjectId{Type: objectType, Instance: objectInstance}, CovOptions{
		ProcessID:  processID,
		Confirmed:  true,
		Indefinite: true,
		Cancel:     cancel,
	})
}

// SubscribeCovObject is SubscribeCovWithProcessID with a full 22-bit object instance.
func (s *Server) SubscribeCovObject(ctx context.Context, deviceIP net.IP, object types.ObjectId, processID uint32, cancel bool) (*CovNotifier, error) {
	if object.InstanceNumber() > types.BacnetMaxInstance {
		return nil, fmt.Errorf("object instance %d exceeds maximum of %d", object.InstanceNumber(), types.BacnetMaxInstance)
	}
	if object.Type >= types.BacnetMaxObject+1 {
		return nil, fmt.Errorf("object type %d exceeds maximum of %d", object.Type, types.BacnetMaxObject)
	}
	return s.subscribeCov(ctx, deviceIP, object, CovOptions{
		ProcessID:  processID,
		Confirmed:  true,
		Indefinite: true,
		Cancel:     cancel,
	})
}

// CovOptions configures a COV subscription request beyond the legacy
// confirmed/indefinite defaults used by SubscribeCov and
// SubscribeCovWithProcessID.
type CovOptions struct {
	ProcessID uint32
	// Confirmed selects confirmed (true) or unconfirmed (false) COV
	// notifications. Ignored when Cancel is set.
	Confirmed bool
	// LifetimeSeconds is the requested subscription lifetime. It is only
	// meaningful when Indefinite is false.
	LifetimeSeconds uint32
	// Indefinite requests a subscription with no defined expiration; when
	// set, the lifetime tag is written with a value of 0 rather than
	// LifetimeSeconds.
	Indefinite bool
	// Cancel requests cancellation of an existing subscription for the
	// same device, object, and ProcessID.
	Cancel bool
}

// SubscribeCovWithOptions subscribes to a monitored object's COV
// notifications using explicit options: confirmed vs. unconfirmed
// notification delivery and an explicit or indefinite lifetime.
func (s *Server) SubscribeCovWithOptions(ctx context.Context, deviceIP net.IP, object types.ObjectId, opts CovOptions) (*CovNotifier, error) {
	if object.InstanceNumber() > types.BacnetMaxInstance {
		return nil, fmt.Errorf("object instance %d exceeds maximum of %d", object.InstanceNumber(), types.BacnetMaxInstance)
	}
	if object.Type >= types.BacnetMaxObject+1 {
		return nil, fmt.Errorf("object type %d exceeds maximum of %d", object.Type, types.BacnetMaxObject)
	}
	return s.subscribeCov(ctx, deviceIP, object, opts)
}

func (s *Server) subscribeCov(ctx context.Context, deviceIP net.IP, object types.ObjectId, opts CovOptions) (*CovNotifier, error) {
	if deviceIP == nil || deviceIP.Equal(net.IP{0, 0, 0, 0}) {
		return nil, errors.New("received a nil or empty device IP")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req := NewRequest()
	defer req.Release()

	handler := make(chan *Request, 128)

	s.SetCovHandlerWithProcessID(deviceIP, opts.ProcessID, handler)
	handlerOwned := false
	defer func() {
		if !handlerOwned {
			s.RemoveCovHandlerWithProcessID(deviceIP, opts.ProcessID)
		}
	}()

	req.SetConfirmedService(types.ConfirmedServiceSubscribeCov, newSubscribeCovPayload(
		object, opts,
	), deviceIP)

	if err := req.Send(deviceIP, s); err != nil {
		return nil, err
	}

	// The first response will be our Ack or Abort/Error/Reject
	// Let's listen to that first and figure out if this request was successful;
	// and if it is, we can start listening for the next Acks and emit the covData.
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
			if !opts.Cancel {
				n := &CovNotifier{
					out:            make(chan *pdu.CovNotification, 128),
					err:            make(chan error, 128),
					subscriptionId: opts.ProcessID,
					handler:        handler,
					deviceIP:       deviceIP,
				}

				handlerOwned = true
				go n.startListening(ctx, s)

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

func newSubscribeCovPayload(object types.ObjectId, opts CovOptions) *pdu.SubscribeCov {
	payload := &pdu.SubscribeCov{
		ObjectId:            &object,
		ProcessIdentifier:   uint8(opts.ProcessID),
		ProcessIdentifier32: opts.ProcessID,
		Cancel:              opts.Cancel,
	}
	if !opts.Cancel {
		payload.IssueConfirmed = opts.Confirmed
		payload.HasLifetime = true
		if !opts.Indefinite {
			payload.Lifetime = opts.LifetimeSeconds
		}
	}
	return payload
}

func (n *CovNotifier) startListening(ctx context.Context, server *Server) {
	defer func() {
		server.RemoveCovHandlerWithProcessID(n.deviceIP, n.subscriptionId)
		for {
			select {
			case data := <-n.handler:
				if data != nil {
					data.Release()
				}
			default:
				return
			}
		}
	}()
	defer close(n.out)
	defer close(n.err)

	for {
		select {
		case data := <-n.handler:
			if data == nil {
				continue
			}
			func() {
				defer data.Release()
				if data.Successful() {
					if val, ok := data.ResponseData().(*pdu.CovNotification); ok {
						// Only confirmed notifications (delivered as a
						// confirmed service request) require a Simple-ACK;
						// unconfirmed notifications get no response.
						if data.PduType() == types.PduTypeConfirmedServiceRequest {
							if err := n.sendAck(server, data); err != nil {
								n.reportError(ctx, err)
								return
							}
						}
						select {
						case n.out <- val:
						case <-ctx.Done():
						}
					} else {
						n.reportError(ctx, errors.New("error decoding response"))
					}
				} else if data.Errored() {
					n.reportError(ctx, errors.New(data.ErrorMessage()))
				} else if data.Aborted() {
					n.reportError(ctx, errors.New(data.AbortReason()))
				} else if data.Rejected() {
					n.reportError(ctx, errors.New(data.RejectReason()))
				} else {
					n.reportError(ctx, errors.New("internal error"))
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (n *CovNotifier) reportError(ctx context.Context, err error) {
	select {
	case n.err <- err:
	case <-ctx.Done():
	default:
	}
}

func (n *CovNotifier) sendAck(server *Server, nReq *Request) error {
	req := NewRequest()
	defer req.Release()

	req.Apdu.ServiceChoice = types.ConfirmedServiceCovNotification
	req.Apdu.RequestData = nil // just in case
	req.Apdu.PduType = types.PduTypeSimpleAck
	req.Apdu.InvokeID = nReq.InvokeID()
	req.Npci.ExpectingReply = false
	req.Npci.IsConfirmed = true
	req.Header.Function = types.BvlcFunctionOriginalUnicastNpdu

	if data, err := req.MarshalBinary(); err == nil {
		return server.Send(data, nReq.Sender)
	} else {
		return err
	}
}
