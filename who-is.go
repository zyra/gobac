package gobac

import (
	"fmt"
	"time"
)

type Res struct{}

type whoIsRequest struct {
	*baseService
	devices *[]*Device
}

// Broadcasts a whois to the network
func SendWhoIsRequest(ifname string) (*[]*Device, error) {
	var instanceMin uint32 = 0
	var instanceMax uint32 = 0x3FFFFF

	req := &whoIsRequest{}

	if baseService, err := newBaseService(ifname); err != nil {
		return nil, err
	} else {
		req.baseService = baseService
	}

	devices := make([]*Device, 0)
	req.devices = &devices

	req.request.EncodeWhoIsApdu(instanceMin, instanceMax)
	req.request.Send()

	req.receiver.Timeout = time.Second * 3

	c := make(chan Response)
	req.receiver.Receive(c)

Loop:
	for {
		select {
		case v := <-c:
			req.handle(v)
			continue

		case <-req.receiver.Done:
			break Loop
		}
	}

	req.waitGroup.Wait()

	return req.devices, nil
}

func (r *whoIsRequest) handle(v Response) {
	r.waitGroup.Add(1)
	go func(r *whoIsRequest, v Response) {
		fmt.Println("Processing iAm response")
		device := NewDevice()

		if err := v.DecodeResponse(device); err != nil {
			fmt.Println("error decoding response", err)
		} else {
			device.OriginInterface = r.ifname
			device.IPAddress = &v.Sender.IP
			r.mutex.Lock()
			*r.devices = append(*r.devices, device)
			r.mutex.Unlock()
		}

		r.waitGroup.Done()
		fmt.Println("Processed iAm response!")
	}(r, v)
}
