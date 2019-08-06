package bacnet

import (
	"github.com/zyra/gobac/bacnet/types"
	"time"
)

func (s *Server) WhoIs(timeout time.Duration) ([]*types.Device, error) {
	req := NewRequest()
	defer req.Cleanup()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)

	tc := time.After(timeout)

	if err := req.Broadcast(s, types.UnconfirmedServiceIAm); err != nil {
		return nil, err
	}

	devices := make([]*types.Device, 0)

Loop:
	for {
		select {
		case <-tc:
			break Loop
		case data := <-req.Data():
			if data.Successful() {
				if val, ok := data.ResponseData().(*types.Device); ok {
					devices = append(devices, val)
				}
			}
			continue
		}
	}

	return devices, nil
}
