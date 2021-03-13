package bacnet

import (
	"context"
	"github.com/zyra/gobac/bacnet/types"
	"sync"
	"time"
)

type WhoIsRequest struct {
	*Request
	devCh  chan *types.Device
	doneCh chan struct{}
	wg     *sync.WaitGroup
}

func (r *WhoIsRequest) Device() <-chan *types.Device {
	return r.devCh
}

func (r *WhoIsRequest) Done() <-chan struct{} {
	return r.doneCh
}

func (s *Server) SendWhoIsBroadcast(ctx context.Context) (*WhoIsRequest, error) {
	req := NewRequest()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)

	if err := req.Broadcast(s, types.UnconfirmedServiceIAm); err != nil {
		return nil, err
	}

	wiReq := WhoIsRequest{
		Request: req,
		devCh:   make(chan *types.Device, 10),
		doneCh:  make(chan struct{}, 0),
		wg:      new(sync.WaitGroup),
	}

	handle := func(data *Request) {
		defer wiReq.wg.Done()
		defer data.Release()
		if data.Successful() {
			if val, ok := data.ResponseData().(*types.Device); ok {
				wiReq.devCh <- val
			}
		}
	}

	go func(wiReq *WhoIsRequest) {
		for {
			select {
			case <-ctx.Done():
				wiReq.wg.Wait()
				close(wiReq.devCh)
				close(wiReq.doneCh)
				wiReq.Release()
				return

			case data := <-wiReq.Data():
				wiReq.wg.Add(1)
				go handle(data)
			}
		}
	}(&wiReq)

	return &wiReq, nil
}

func (s *Server) WhoIs(ctx context.Context, timeout time.Duration) (<-chan *types.Device, error) {
	bCtx, _ := context.WithTimeout(ctx, timeout)

	req, err := s.SendWhoIsBroadcast(bCtx)

	if err != nil {
		return nil, err
	}

	return req.Device(), nil
}
