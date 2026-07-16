package bacnet

import (
	"bytes"
	"encoding"
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"sync"
)

var reqPool = sync.Pool{
	New: func() interface{} {
		req := &Request{
			Npci:   &pdu.Npci{},
			Header: &types.Header{},
			Apdu:   &pdu.Apdu{},
		}
		req.Reset()
		return req
	},
}

var buffPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer([]byte{})
	},
}

type Request struct {
	Header *types.Header
	Npci   *pdu.Npci
	Apdu   *pdu.Apdu
	tx     chan *Request
	//done   chan struct{}
	//err    chan error
	Sender *net.UDPAddr

	server             *Server
	invokeAddress      net.IP
	unconfirmedService types.UnconfirmedService
	unconfirmedToken   uint64
}

func (r *Request) Reset() {
	*r.Header = types.Header{ProtocolType: types.BACnetProtocol}
	r.Npci.Reset()
	*r.Apdu = pdu.Apdu{MaxApdu: types.MaxApdu}
	r.tx = nil
	r.Sender = nil
	r.server = nil
	r.invokeAddress = nil
	r.unconfirmedService = 0
	r.unconfirmedToken = 0
}

func (r *Request) Release() {
	r.cleanupTransaction()
	r.releaseQueuedResponses()
	r.Reset()
	reqPool.Put(r)
}

func NewRequest() *Request {
	return reqPool.Get().(*Request)
}

func ParseRequest(b []byte, sender *net.UDPAddr) (*Request, error) {
	if sender == nil {
		return nil, errors.New("sender cannot be nil")
	}
	req := NewRequest()
	req.Sender = sender
	if err := req.UnmarshalBinary(b); err != nil {
		req.Release()
		return nil, err
	}
	return req, nil
}

func (r *Request) SetConfirmedService(choice types.ConfirmedService, data encoding.BinaryMarshaler, address net.IP) {
	r.cleanupTransaction()
	r.releaseQueuedResponses()
	r.Reset()
	r.Apdu.ServiceChoice = choice
	r.Apdu.PduType = types.PduTypeConfirmedServiceRequest
	r.Apdu.RequestData = data
	r.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	r.Npci.IsConfirmed = true
	r.Npci.ExpectingReply = true
	r.Apdu.InvokeID, _ = TryGetInvokeID(address)
	r.invokeAddress = append(net.IP(nil), address...)
	r.tx = make(chan *Request, 1)
	//r.err = make(chan error)
}

func (r *Request) SetUnconfirmedService(choice types.UnconfirmedService, data encoding.BinaryMarshaler) {
	r.cleanupTransaction()
	r.releaseQueuedResponses()
	r.Reset()
	r.Apdu.ServiceChoice = choice
	r.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	r.Apdu.RequestData = data
	r.Npci.DestinationNet = 65535
	r.Header.Function = types.BvlcFunctionOriginalBroadcastNpdu
	r.tx = make(chan *Request, 10)
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
	if server == nil {
		return errors.New("server cannot be nil")
	}
	if data, err := r.MarshalBinary(); err != nil {
		return err
	} else {
		r.server = server
		r.unconfirmedService = responseChoice
		r.unconfirmedToken = server.addUnconfirmedHandler(responseChoice, r.tx)

		if err := server.Send(data, server.GetBroadcastAddr()); err != nil {
			r.cleanupTransaction()
			return err
		}
	}

	return nil
}

func (r *Request) Send(dest net.IP, server *Server) error {
	if server == nil {
		r.cleanupTransaction()
		return errors.New("server cannot be nil")
	}
	if dest == nil {
		r.cleanupTransaction()
		return errors.New("destination cannot be nil")
	}
	if r.InvokeID() == 0 {
		return ErrInvokeIDExhausted
	}

	destUdp := getUdpAddr(dest, server.GetBroadcastPort())

	if data, err := r.MarshalBinary(); err != nil {
		r.cleanupTransaction()
		return err
	} else {
		r.server = server
		server.SetConfirmedHandler(dest, r.InvokeID(), r.tx)

		if err := server.Send(data, destUdp); err != nil {
			r.cleanupTransaction()
			return err
		}
	}

	return nil
}

func (r *Request) releaseQueuedResponses() {
	if r.tx == nil {
		return
	}
	for {
		select {
		case response := <-r.tx:
			if response != nil && response != r {
				response.Release()
			}
		default:
			return
		}
	}
}

func (r *Request) cleanupTransaction() {
	if r.server != nil && r.InvokeID() != 0 && r.invokeAddress != nil {
		r.server.RemoveConfirmedHandler(r.invokeAddress, r.InvokeID())
	}
	if r.InvokeID() != 0 && r.invokeAddress != nil {
		ReleaseInvokeID(r.invokeAddress, r.InvokeID())
	}
	if r.server != nil && r.unconfirmedToken != 0 {
		r.server.removeUnconfirmedHandler(r.unconfirmedService, r.unconfirmedToken)
	}
	r.server = nil
	r.invokeAddress = nil
	r.unconfirmedToken = 0
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

	buff := buffPool.Get().(*bytes.Buffer)
	buff.Write(b)
	defer buff.Reset()
	defer buffPool.Put(buff)

	if err := r.Header.UnmarshalBinary(buff.Next(4)); err != nil {
		return err
	}
	if int(r.Header.BvlcLength) != len(b) {
		return errors.New("BVLC length does not match datagram length")
	}

	if b := buff.Bytes(); len(b) != int(r.Header.NsduLength) {
		return errors.New("byte slice is too short")
	} else if err := r.Npci.UnmarshalBinary(buff.Bytes()); err != nil {
		return err
	}

	// mark bytes as read
	buff.Next(r.Npci.Length)

	r.Apdu.SenderIP = r.Sender.IP
	r.Apdu.BacnetPort = uint16(r.Sender.Port)

	if err := r.Apdu.UnmarshalBinary(buff.Bytes()); err != nil {
		return err
	}

	return nil
}
