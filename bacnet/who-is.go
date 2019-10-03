package bacnet

import (
	"context"
	"github.com/zyra/gobac/bacnet/types"
	"sync"
	"time"
)

func (s *server) WhoIs(ctx context.Context, timeout time.Duration) (<-chan *types.Device, error) {
	req := NewRequest()
	defer req.Release()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)

	if err := req.Broadcast(s, types.UnconfirmedServiceIAm); err != nil {
		return nil, err
	}

	dChan := make(chan *types.Device, 10)

	wg := sync.WaitGroup{}

	handle := func(data *Request) {
		defer wg.Done()
		defer data.Release()
		if data.Successful() {
			if val, ok := data.ResponseData().(*types.Device); ok {
				dChan <- val
			}
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(dChan)
				return

			case <-time.After(timeout):
				wg.Wait()
				close(dChan)
				return

			case data := <-req.Data():
				wg.Add(1)
				go handle(data)
			}
		}
	}()

	return dChan, nil
}
