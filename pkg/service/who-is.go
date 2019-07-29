package service

import (
	"fmt"
	"github.com/zyra/bacnet-2/pkg/object"
	"github.com/zyra/bacnet-2/pkg/pdu"
	"github.com/zyra/bacnet-2/pkg/util"
	"sync"
	"time"
)

type Res struct{}

type WhoIsRequest struct {
	mutex          sync.RWMutex
	waitGroup      sync.WaitGroup
	ifname         string
	Devices        *[]*object.Device
}

// Broadcasts a whois to the network
func SendWhoIsRequest(ifname string) (*[]*object.Device, error) {
	ipHelper, err := util.NewIPHelper(ifname)

	if err != nil {
		return nil, err
	}

	var instanceMin uint32 = 0
	var instanceMax uint32 = 0x3FFFFF

	req := &WhoIsRequest{}
	devices := make([]*object.Device, 0)
	req.Devices = &devices
	req.ifname = ifname

	request := pdu.NewRequest()
	request.Source = ipHelper.IPv4
	request.SourcePort = 0xBAC0
	request.Target = ipHelper.BroadcastIPv4
	request.TargetPort = 0xBAC0
	request.EncodeNpdu()
	request.EncodeWhoIsApdu(instanceMin, instanceMax)
	request.Send()

	response := pdu.NewPduReceiver(ipHelper.IPv4, 0xBAC0)
	response.Target = ipHelper.BroadcastIPv4
	response.TargetPort = 0xBAC0
	response.Timeout = time.Second * 3

	c := make(chan pdu.Response)
	response.Receive(c)

Loop:
	for {
		select {
		case v := <-c:
			req.handle(v)
			continue

		case <-response.Done:
			break Loop
		}
	}

	req.waitGroup.Wait()

	return req.Devices, nil
}

func (r *WhoIsRequest) handle(v pdu.Response) {
	r.waitGroup.Add(1)
	go func(r *WhoIsRequest, v pdu.Response) {
		fmt.Println("Processing iAm response")
		var device object.Device
		if err := v.DecodeResponse(&device); err != nil {
			fmt.Println("error decoding response", err)
		} else {
			device.OriginInterface = r.ifname
			device.IPAddress = &v.Sender.IP
			r.mutex.Lock()
			*r.Devices = append(*r.Devices, &device)
			r.mutex.Unlock()
		}

		r.waitGroup.Done()
		fmt.Println("Processed iAm response!")
	}(r, v)
}
