package pdu

import (
	"fmt"
	"github.com/zyra/bacnet-2/pkg/type"
	"github.com/zyra/bacnet-2/pkg/util"
	"net"
)

func NewRequest() *Request {
	d := &Request{
		Pdu: NewPdu(),
	}
	return d
}

type Request struct {
	*Pdu
}

func (d *Request) EncodeNpdu() {
	var b byte

	d.Append(d.ProtocolVersion)

	b = 0

	if d.NetworkLayerMessage {
		b |= _type.BIT7
	}

	b |= _type.BIT5

	if d.ExpectingReply {
		b |= _type.BIT2
	}

	b |= d.Priority & 0x03

	d.Append(b)

	d.EncodeUnsigned16(65535)
	d.Append(0)
	d.Append(d.HopCount)

	if d.NetworkLayerMessage {
		if d.NetworkMessageType > 255 {
			panic("Invalid message type")
		}

		d.Append(byte(d.NetworkMessageType))

		if d.NetworkMessageType >= 0x80 {
			d.Append(
				byte((d.VendorID&0xff00)>>8),
				byte(d.VendorID&0x00ff),
			)
		}
	}
}

func (d *Request) EncodeWhoIsApdu(minInstance, maxInstance uint32) {
	d.Append(
		_type.PDU_TYPE_UNCONFIRMED_SERVICE_REQUEST,
		_type.SERVICE_UNCONFIRMED_WHO_IS,
	)

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

	d.EncodeTag(tag, true, tagLen)
	d.EncodeUnsigned(value)
}

func (d *Request) Send() {
	buff := NewBuffer()

	buff.Append(0x81, _type.BVLC_ORIGINAL_BROADCAST_NPDU)
	buff.EncodeUnsigned16(uint16(d.Len()) + 4)
	buff.AppendArray(d.Bytes())
	d.SendMDPU(buff)
}

func (d *Request) SendMDPU(mtu *Buffer) {
	srcUdp := util.GetUdpAddr(d.Source, d.SourcePort)
	destUdp := util.GetUdpAddr(d.Target, d.TargetPort)

	if conn, err := net.DialUDP("udp", srcUdp, destUdp); err != nil {
		fmt.Println("Error dialing UDP", err)
	} else {
		defer func() {
			if err := conn.Close(); err != nil {
				fmt.Println("Error closing UDP connection", err)
			}
		}()
		if _, err := conn.Write(mtu.Bytes()); err != nil {
			fmt.Println("Error sending MDPU", err)
		}
	}
}
