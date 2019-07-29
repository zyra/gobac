package gobac

import (
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"net"
)

type Pdu struct {
	*encoding.Buffer
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
	return &Pdu{
		Buffer:              encoding.NewBuffer(),
		ProtocolVersion:     1,
		ExpectingReply:      false,
		NetworkLayerMessage: false,
		Priority:            types.MESSAGE_PRIORITY_NORMAL,
		NetworkMessageType:  types.NETWORK_MESSAGE_INVALID,
		VendorID:            0,
		HopCount:            255,
	}
}
