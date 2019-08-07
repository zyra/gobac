package bacnet

import (
	"github.com/zyra/gobac/bacnet/types"
	"time"
)

func (s *Server) WhoIs(timeout time.Duration) (<-chan *types.Device, error) {
	req := NewRequest()
	defer req.Cleanup()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)

	tc := time.After(timeout)

	if err := req.Broadcast(s, types.UnconfirmedServiceIAm); err != nil {
		return nil, err
	}

	dChan := make(chan *types.Device, 128)

	go func() {
	Loop:
		for {
			select {
			case <-tc:
				close(dChan)
				break Loop
			case data := <-req.Data():
				if data.Successful() {
					if val, ok := data.ResponseData().(*types.Device); ok {
						dChan <- val
					}
				}
				continue
			}
		}
	}()

	return dChan, nil
}
