package bacnet

import (
	"bytes"
	"encoding"
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
)

type Request struct {
	Header *types.Header
	Npci   *pdu.Npci
	Apdu   *pdu.Apdu
	tx     chan *Request
	//done   chan struct{}
	//err    chan error
	sender net.IP
}

func NewRequest() *Request {
	return &Request{
		Header: &types.Header{
			ProtocolType: types.BACnetProtocol,
		},
		Npci: &pdu.Npci{
			ProtocolVersion:     types.BACnetVersion,
			NetworkLayerMessage: false,
			IsConfirmed:         false,
			ExpectingReply:      false,
			Priority:            types.MessagePriorityNormal,
			HopCount:            255,
		},
		Apdu: &pdu.Apdu{
			MaxSegments: 0,
			MaxApdu:     types.MaxApdu,
		},
	}
}

func ParseRequest(b []byte, sender net.IP) (*Request, error) {
	req := NewRequest()
	req.sender = sender
	return req, req.UnmarshalBinary(b)
}

func (r *Request) SetConfirmedService(choice types.ConfirmedService, data encoding.BinaryMarshaler) {
	r.Apdu.ServiceChoice = choice
	r.Apdu.PduType = types.PduTypeConfirmedServiceRequest
	r.Apdu.RequestData = data
	r.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	r.Npci.IsConfirmed = true
	r.Npci.ExpectingReply = true
	r.Apdu.InvokeID = GetInvokeID()
	r.tx = make(chan *Request)
	//r.err = make(chan error)
}

func (r *Request) SetUnconfirmedService(choice types.UnconfirmedService, data encoding.BinaryMarshaler) {
	r.Apdu.ServiceChoice = choice
	r.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	r.Apdu.RequestData = data
	r.Npci.DestinationNet = 65535
	r.Header.Function = types.BvlcFunctionOriginalBroadcastNpdu
	r.tx = make(chan *Request, 128)
	//r.err = make(chan error, 128)
}

func (r *Request) InvokeID() uint8 {
	return r.Apdu.InvokeID
}

func (r *Request) ResponseData() encoding.BinaryUnmarshaler {
	return r.Apdu.ResponseData
}

func (r *Request) PduType() types.PduType {
	return r.Apdu.PduType
}

func (r *Request) ServiceChoice() uint8 {
	return r.Apdu.ServiceChoice
}

func (r *Request) Broadcast(server *Server, responseChoice types.UnconfirmedService) error {
	if data, err := r.MarshalBinary(); err != nil {
		return err
	} else {
		server.SetUnconfirmedHandler(responseChoice, r.tx)

		if err := server.Send(data, server.BroadcastAddr); err != nil {
			return err
		}
	}

	return nil
}

func (r *Request) Send(dest net.IP, server *Server) error {
	destUdp := getUdpAddr(dest, server.BroadcastPort)

	if data, err := r.MarshalBinary(); err != nil {
		return err
	} else {
		server.SetConfirmedHandler(r.InvokeID(), r.tx)

		if err := server.Send(data, destUdp); err != nil {
			return err
		}
	}

	return nil
}

func (r *Request) Cleanup() {
	//ReleaseInvokeID(r.Apdu.InvokeID)
	//close(r.tx)
	//close(r.err)
	//close(r.done)
}

func (r *Request) Data() <-chan *Request {
	return r.tx
}

func (r *Request) Successful() bool {
	return !r.Apdu.Failed
}

func (r *Request) Errored() bool {
	return r.Apdu.Errored
}

func (r *Request) ErrorMessage() string {
	return r.Apdu.ErrorClass.String() + " " + r.Apdu.ErrorCode.String()
}

func (r *Request) Aborted() bool {
	return r.Apdu.Aborted
}

func (r *Request) AbortReason() string {
	return r.Apdu.AbortReason.String()
}

func (r *Request) Rejected() bool {
	return r.Apdu.Rejected
}

func (r *Request) RejectReason() string {
	return r.Apdu.RejectReason.String()
}

//
//func (r *Request) Error() <-chan error {
//	return r.err
//}
//
//func (r *Request) Done() <-chan struct{} {
//	return r.done
//}

func (r *Request) MarshalBinary() ([]byte, error) {
	apduCommonBytes, err := r.Apdu.MarshalBinary()

	if err != nil {
		return nil, err
	}

	npciBytes, err := r.Npci.MarshalBinary()

	if err != nil {
		return nil, err
	}

	commonApduLen := len(apduCommonBytes)
	npciLen := len(npciBytes)

	r.Header.NsduLength = types.Uint16(commonApduLen + npciLen)
	r.Header.BvlcLength = r.Header.NsduLength + 4

	headerBytes, err := r.Header.MarshalBinary()

	if err != nil {
		return nil, err
	}

	b := make([]byte, r.Header.BvlcLength)

	copy(b, headerBytes)
	copy(b[4:], npciBytes)
	copy(b[4+npciLen:], apduCommonBytes)

	return b, nil
}

func (r *Request) UnmarshalBinary(b []byte) error {
	if len(b) < 6 {
		return errors.New("byte slice is too short")
	}

	buff := bytes.NewBuffer(b)

	if err := r.Header.UnmarshalBinary(buff.Next(4)); err != nil {
		return err
	}

	if b := buff.Bytes(); len(b) < int(r.Header.NsduLength) {
		return errors.New("byte slice is too short")
	} else if err := r.Npci.UnmarshalBinary(buff.Bytes()); err != nil {
		return err
	}

	// mark bytes as read
	buff.Next(r.Npci.Length)

	r.Apdu.SenderIP = r.sender

	if err := r.Apdu.UnmarshalBinary(buff.Bytes()); err != nil {
		return err
	}

	return nil
}
