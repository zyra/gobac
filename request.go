package gobac

import (
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"log"
)

type Request struct {
	Pdu
	Server      *Server
	IsConfirmed bool
	err         chan error
	data        chan *Response
	done        chan struct{}
}

func (s *Server) NewRequest() Request {
	req := Request{
		Pdu:    NewPdu(),
		Server: s,
		done:   make(chan struct{}),
	}

	req.Source = s.IPv4
	req.SourcePort = s.ServerPort
	req.Target = s.BroadcastIPv4
	req.TargetPort = s.BroadcastPort
	return req
}

func (d *Request) EncodeNPCI() {
	var b byte

	_ = d.AppendByte(d.ProtocolVersion)

	b = 0

	if d.NetworkLayerMessage {
		b |= types.BIT7
	}

	if !d.IsConfirmed {
		b |= types.BIT5
	}

	if d.ExpectingReply {
		b |= types.BIT2
	}

	b |= d.Priority & 0x03

	_ = d.AppendByte(b)

	// Broadcast
	if !d.IsConfirmed {
		_ = d.EncodeUnsigned16(65535)
		_ = d.AppendByte(0)
		_ = d.AppendByte(d.HopCount)
	}

	if d.NetworkLayerMessage {
		log.Println("encoding NPDU with a network layer message; this is not supported!")
	}
}

func (d *Request) EncodeHeaders() error {
	buff := encoding.NewBuffer()

	err := buff.AppendByte(0x81)

	if d.IsConfirmed {
		err = buff.AppendByte(types.BVLC_ORIGINAL_UNICAST_NPDU)
	} else {
		err = buff.AppendByte(types.BVLC_ORIGINAL_BROADCAST_NPDU)
	}

	err = buff.EncodeUnsigned16(uint16(len(d.Bytes())) + 4)
	err = buff.AppendBytes(d.Bytes())

	d.Buffer = buff

	return err
}

func (d *Request) Send() error {
	destUdp := getUdpAddr(d.Target, d.TargetPort)
	if err := d.EncodeHeaders(); err != nil {
		return err
	}
	return d.Server.Send(d.Buffer.Bytes(), destUdp)
}

func (d *Request) closeChans() {
	close(d.err)
	close(d.data)
	close(d.done)
}

func (d *Request) Data() <-chan *Response {
	return d.data
}

func (d *Request) Error() <-chan error {
	return d.err
}

func (d *Request) Done() <-chan struct{} {
	return d.done
}
