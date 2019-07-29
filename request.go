package gobac

import (
	"bytes"
	"fmt"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"github.com/zyra/gobac/util"
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

	_ = encoding.AppendByte(d.Buffer, d.ProtocolVersion)

	b = 0

	if d.NetworkLayerMessage {
		b |= types.BIT7
	}

	b |= types.BIT5

	if d.ExpectingReply {
		b |= types.BIT2
	}

	b |= d.Priority & 0x03

	_ = encoding.AppendByte(d.Buffer, b)
	_ = encoding.EncodeUnsigned16(d.Buffer, 65535)
	_ = encoding.AppendByte(d.Buffer, 0)
	_ = encoding.AppendByte(d.Buffer, d.HopCount)

	if d.NetworkLayerMessage {
		if d.NetworkMessageType > 255 {
			panic("Invalid message type")
		}

		_ = encoding.AppendByte(d.Buffer, byte(d.NetworkMessageType))

		if d.NetworkMessageType >= 0x80 {
			_ = encoding.AppendBytes(d.Buffer, []byte{
				byte((d.VendorID & 0xff00) >> 8),
				byte(d.VendorID & 0x00ff),
			})
		}
	}
}

func (d *Request) EncodeWhoIsApdu(minInstance, maxInstance uint32) {
	_ = encoding.AppendBytes(d.Buffer, []byte{
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

	_ = encoding.EncodeTag(d.Buffer, tag, true, tagLen)
	_ = encoding.EncodeUnsigned(d.Buffer, value)
}

func (d *Request) Send() {
	b := make([]byte, 0)
	b[0] = 0x81
	b[1] = types.BVLC_ORIGINAL_BROADCAST_NPDU
	buff := bytes.NewBuffer(b)
	_ = encoding.EncodeUnsigned16(buff, uint16(d.Len())+4)
	_ = encoding.AppendBytes(buff, d.Bytes())
	d.SendMDPU(buff)
}

func (d *Request) SendMDPU(mtu *bytes.Buffer) {
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
