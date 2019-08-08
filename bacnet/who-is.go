package bacnet

import (
	"github.com/zyra/gobac/bacnet/types"
	"sync"
	"time"
)

func (s *Server) WhoIs(timeout time.Duration) (<-chan *types.Device, error) {
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
	Loop:
		for {
			select {
			case <-time.After(timeout):
				wg.Wait()
				close(dChan)
				break Loop
			case data := <-req.Data():
				wg.Add(1)
				go handle(data)
				continue
			}
		}
	}()

	return dChan, nil
}
