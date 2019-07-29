package service

import (
	"github.com/zyra/bacnet-2/pkg/pdu"
	"github.com/zyra/bacnet-2/pkg/util"
	"sync"
)

type baseService struct {
	mutex     sync.RWMutex
	waitGroup sync.WaitGroup
	ifname    string
	ipHelper  *util.IPHelper
	request   *pdu.Request
	receiver  *pdu.Receiver
	invokeId  uint8
}

func newBaseService(ifname string) (*baseService, error) {
	ipHelper, err := util.NewIPHelper(ifname)

	if err != nil {
		return nil, err
	}

	s := &baseService{
		ifname:   ifname,
		ipHelper: ipHelper,
	}

	req := pdu.NewRequest()
	req.Source = ipHelper.IPv4
	req.SourcePort = 0xBAC0
	req.Target = ipHelper.BroadcastIPv4
	req.TargetPort = 0xBAC0
	req.EncodeNpdu()

	rec := pdu.NewPduReceiver(req.Source, req.SourcePort)
	rec.Target = req.Target
	rec.TargetPort = req.TargetPort

	s.request = req
	s.receiver = rec

	return s, nil
}

func setInvokeId() {

}
