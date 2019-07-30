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
	Priority            MessagePriority
	VendorID            uint16
	HopCount            uint8

	ProtocolType  uint8
	Function      BVLCFunction
	MessageLength uint16
	ControlOctet  byte
	BVLCLength    uint16
	NPDULength    uint16
	ServiceChoice uint8
	PduType       PduType
	InvokeID      uint8
}

func NewPdu() *Pdu {
	return &Pdu{
		Buffer:              encoding.NewBuffer(),
		ProtocolVersion:     1,
		ExpectingReply:      false,
		NetworkLayerMessage: false,
		Priority:            types.MESSAGE_PRIORITY_NORMAL,
		VendorID:            0,
		HopCount:            255,
	}
}
