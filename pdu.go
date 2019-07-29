package gobac

import (
	"bytes"
	"github.com/zyra/gobac/types"
	"net"
)

type Pdu struct {
	*bytes.Buffer
	Source              *net.IP
	SourcePort          uint16
	Target              *net.IP
	TargetPort          uint16
	ProtocolVersion     uint8
	ExpectingReply      bool
	NetworkLayerMessage bool
	Priority            types.MessagePriority
	NetworkMessageType  types.NetworkMessageType
	VendorID            uint16
	HopCount            uint8
}

func NewPdu() *Pdu {
	b := make([]byte, 0)
	return &Pdu{
		Buffer:              bytes.NewBuffer(b),
		ProtocolVersion:     1,
		ExpectingReply:      false,
		NetworkLayerMessage: false,
		Priority:            types.MESSAGE_PRIORITY_NORMAL,
		NetworkMessageType:  types.NETWORK_MESSAGE_INVALID,
		VendorID:            0,
		HopCount:            255,
	}
}
