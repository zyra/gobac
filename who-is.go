package gobac

import (
	"fmt"
	"github.com/zyra/gobac/types"
	"sync"
	"time"
)

type whoIsRequest struct {
	*serviceRequest
	devices   *[]*Device
	mutex     sync.RWMutex
	waitGroup sync.WaitGroup
}

func (s *Server) WhoIs(dest *[]*Device) error {
	var instanceMin uint32 = 0
	var instanceMax uint32 = 0x3FFFFF

	req := &whoIsRequest{
		devices:        dest,
		serviceRequest: newServiceRequest(s),
	}

	req.request.EncodeWhoIsApdu(instanceMin, instanceMax)
	req.request.Send()

	tc, c, h := getChanHandlerWithTimeout(time.Second * 5)

	s.setUnconfirmedHandler(types.SERVICE_UNCONFIRMED_I_AM, h)

Loop:
	for {
		select {
		case <-tc:
			break Loop
		case data := <-c:
			req.handle(data)
			continue
		}
	}

	req.waitGroup.Wait()

	return nil
}

func (r *whoIsRequest) handle(v *Response) {
	r.waitGroup.Add(1)
	go func(r *whoIsRequest, v *Response) {
		device := NewDevice()
		req := IAmServiceRequest(v.Message.Bytes())

		if err := req.Decode(device); err != nil {
			fmt.Println("error decoding response", err)
		} else {
			device.OriginInterface = r.server.InterfaceName
			device.IPAddress = &v.Sender.IP
			r.mutex.Lock()
			*r.devices = append(*r.devices, device)
			r.mutex.Unlock()
		}

		r.waitGroup.Done()
	}(r, v)
}
