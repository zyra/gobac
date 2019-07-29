package gobac

import (
	"fmt"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
)

func NewRequest(s *Server) *Request {
	req := &Request{
		Pdu:    NewPdu(),
		Server: s,
	}

	req.Source = s.IPv4
	req.SourcePort = s.ServerPort
	req.Target = s.BroadcastIPv4
	req.TargetPort = s.BroadcastPort

	return req
}

type Request struct {
	*Pdu
	Server *Server
}

func (d *Request) EncodeNpdu() {
	var b byte

	_ = d.AppendByte(d.ProtocolVersion)

	b = 0

	if d.NetworkLayerMessage {
		b |= types.BIT7
	}

	b |= types.BIT5

	if d.ExpectingReply {
		b |= types.BIT2
	}

	b |= d.Priority & 0x03

	_ = d.AppendByte(b)

	// Broadcast
	_ = d.EncodeUnsigned16(65535)

	_ = d.AppendByte(0)

	_ = d.AppendByte(d.HopCount)

	if d.NetworkLayerMessage {
		if d.NetworkMessageType > 255 {
			panic("Invalid message type")
		}

		_ = d.AppendByte(byte(d.NetworkMessageType))

		if d.NetworkMessageType >= 0x80 {
			_ = d.AppendBytes([]byte{
				byte((d.VendorID & 0xff00) >> 8),
				byte(d.VendorID & 0x00ff),
			})
		}
	}
}

func (d *Request) EncodeWhoIsApdu(minInstance, maxInstance uint32) {
	_ = d.AppendBytes([]byte{
		types.PDU_TYPE_UNCONFIRMED_SERVICE_REQUEST,
		types.SERVICE_UNCONFIRMED_WHO_IS,
	})

	//d.EncodeContext(0, minInstance)
	//d.EncodeContext(1, maxInstance)
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
	_ = buff.AppendByte(types.BVLC_ORIGINAL_BROADCAST_NPDU)
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
