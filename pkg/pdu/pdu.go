package pdu

import (
	"github.com/zyra/bacnet-2/pkg/type"
	"net"
)

type Pdu struct {
	*Buffer
	Source              *net.IP
	SourcePort          uint16
	Target              *net.IP
	TargetPort          uint16
	ProtocolVersion     uint8
	ExpectingReply      bool
	NetworkLayerMessage bool
	Priority            _type.MessagePriority
	NetworkMessageType  _type.NetworkMessageType
	VendorID            uint16
	HopCount            uint8
}

func NewPdu() *Pdu {
	return &Pdu{
		Buffer:              NewBuffer(),
		ProtocolVersion:     1,
		ExpectingReply:      false,
		NetworkLayerMessage: false,
		Priority:            _type.MESSAGE_PRIORITY_NORMAL,
		NetworkMessageType:  _type.NETWORK_MESSAGE_INVALID,
		VendorID:            0,
		HopCount:            255,
	}
}
