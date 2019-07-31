package gobac

import (
	"fmt"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"log"
)

type Request struct {
	*Pdu
	Server            *Server
	IsBroadcastTarget bool
}

func NewRequest(s *Server) *Request {
	req := &Request{
		Pdu:               NewPdu(),
		Server:            s,
		IsBroadcastTarget: true,
	}

	req.Source = s.IPv4
	req.SourcePort = s.ServerPort
	req.Target = s.BroadcastIPv4
	req.TargetPort = s.BroadcastPort

	return req
}

func (d *Request) EncodeNpdu() {
	var b byte

	_ = d.AppendByte(d.ProtocolVersion)

	b = 0

	if d.NetworkLayerMessage {
		b |= types.BIT7
	}

	if d.IsBroadcastTarget {
		b |= types.BIT5
	}

	if d.ExpectingReply {
		b |= types.BIT2
	}

	b |= d.Priority & 0x03

	_ = d.AppendByte(b)

	// Broadcast
	if d.IsBroadcastTarget {
		_ = d.EncodeUnsigned16(65535)
		_ = d.AppendByte(0)
		_ = d.AppendByte(d.HopCount)
	}

	if d.NetworkLayerMessage {
		log.Println("encoding NPDU with a network layer message; this is not supported!")
	}
}

func (d *Request) EncodeContext(tag uint8, value uint32) {
	var tagLen uint32 = 0

	if value < 0x100 {
		tagLen = 1
	} else if value < 0x10000 {
		tagLen = 2
	} else if value < 0x1000000 {
		tagLen = 3
	} else {
		tagLen = 4
	}

	_ = d.EncodeTag(tag, true, tagLen)
	_ = d.EncodeUnsigned(value)
}

func (d *Request) Send() {
	buff := encoding.NewBuffer()

	_ = buff.AppendByte(0x81)

	if d.IsBroadcastTarget {
		_ = buff.AppendByte(types.BVLC_ORIGINAL_BROADCAST_NPDU)
	} else {
		_ = buff.AppendByte(types.BVLC_ORIGINAL_UNICAST_NPDU)
	}

	_ = buff.EncodeUnsigned16(uint16(d.Len()) + 4)
	_ = buff.AppendBytes(d.Bytes())

	d.SendMDPU(buff)
}

func (d *Request) SendMDPU(mtu *encoding.Buffer) {
	destUdp := getUdpAddr(d.Target, d.TargetPort)

	if err := d.Server.SendMPDU(mtu, destUdp); err != nil {
		fmt.Println("Error sending MDPU", err)
	}
}
