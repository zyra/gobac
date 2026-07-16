package bacnet

import (
	"context"
	"github.com/zyra/gobac/v2/bacnet/types"
	"time"
)

type WhoIsRequest struct {
	*Request
	devCh  chan *types.Device
	doneCh chan struct{}
}

func (r *WhoIsRequest) Device() <-chan *types.Device {
	return r.devCh
}

func (r *WhoIsRequest) Done() <-chan struct{} {
	return r.doneCh
}

func (s *Server) SendWhoIsBroadcast(ctx context.Context) (*WhoIsRequest, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req := NewRequest()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)

	if err := req.Broadcast(s, types.UnconfirmedServiceIAm); err != nil {
		req.Release()
		return nil, err
	}

	wiReq := WhoIsRequest{
		Request: req,
		devCh:   make(chan *types.Device, 10),
		doneCh:  make(chan struct{}),
	}

	go func(wiReq *WhoIsRequest) {
		defer close(wiReq.devCh)
		defer close(wiReq.doneCh)
		defer wiReq.Release()
		for {
			select {
			case <-ctx.Done():
				return

			case data := <-wiReq.Data():
				if data == nil {
					continue
				}
				if data.Successful() {
					if val, ok := data.ResponseData().(*types.Device); ok {
						select {
						case wiReq.devCh <- val:
						case <-ctx.Done():
							data.Release()
							return
						}
					}
				}
				data.Release()
			}
		}
	}(&wiReq)

	return &wiReq, nil
}

func (s *Server) WhoIs(ctx context.Context, timeout time.Duration) (<-chan *types.Device, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	bCtx, cancel := context.WithTimeout(ctx, timeout)

	req, err := s.SendWhoIsBroadcast(bCtx)

	if err != nil {
		cancel()
		return nil, err
	}
	go func() {
		<-req.Done()
		cancel()
	}()

	return req.Device(), nil
}
