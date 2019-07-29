package gobac

import (
	"github.com/kataras/iris/core/errors"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"net"
)

type Response struct {
	*Pdu
	RawData []byte
	Valid   bool
	Sender  *net.UDPAddr
	PduType types.PduType
	Message *encoding.Buffer
	Choice  byte
}

func NewPduResponse(data []byte) *Response {
	pdu := &Response{
		Pdu:     NewPdu(),
		RawData: data,
	}

	pdu.Pdu.Buffer = encoding.NewBuffer(data)

	return pdu
}

func (r *Response) Decode() error {
	data := r.Bytes()

	funct := data[1]

	npduLen := (uint16(data[2]) << 8) & 0xFF00
	npduLen |= uint16(data[3]) & 0x00FF

	npduLen -= 4
	data = data[:npduLen+4]

	switch funct {
	case types.BVLC_ORIGINAL_BROADCAST_NPDU:
		for i := uint16(0); i < npduLen; i++ {
			data[i] = data[4+i]
		}
		break

	default:
		return errors.New("unsupported function type")
	}

	offset := 0

	r.ProtocolVersion = data[0]

	// Make sure protocol is correct
	if r.ProtocolVersion != 1 {
		// Invalid version
		return errors.New("invalid protocol version")
	}

	metaByte := data[1]

	r.NetworkLayerMessage = metaByte&types.BIT7 == 1
	r.ExpectingReply = metaByte&types.BIT2 == 1
	r.Priority = metaByte & 0x03

	offset = 2

	//var srcNet uint16
	var destNet uint16

	//var addrLen uint8

	// Check destination
	if metaByte&types.BIT5 == 1 {
		destNet = encoding.DecodeUnsigned16(data[offset : offset+2])
		offset += 2
		addrLen := data[offset]
		offset++
		offset += int(addrLen)
	}

	// Check source
	if metaByte&types.BIT3 == 1 {
		//srcNet = util.DecodeUnsigned16(data[offset : offset+2])
		offset += 2
		addrLen := data[offset]
		offset++
		offset += int(addrLen)
	}

	// Check hop count
	if destNet > 0 {
		r.HopCount = data[offset]
		offset++
	} else {
		r.HopCount = 0
	}

	if r.NetworkLayerMessage {
		nmt := data[offset]
		offset++

		r.NetworkMessageType = types.NetworkMessageType(nmt)

		if r.NetworkMessageType >= 0x80 {
			r.VendorID = encoding.DecodeUnsigned16(data[offset : offset+2])
			offset += 2
		}
	} else {
		r.NetworkMessageType = types.NETWORK_MESSAGE_INVALID
	}

	if r.NetworkLayerMessage {
		return errors.New("network layer messages are not supported")
	}

	offset += 4
	pduType := data[offset] & 0xF0
	offset++
	choice := data[offset]
	offset++
	request := data[offset:]

	r.Valid = true
	r.PduType = pduType
	r.Message = encoding.NewBuffer(request)
	r.Choice = choice

	return nil
}

func (r *Response) HandleUnconfirmedServiceRequest(choice byte, request []byte, dest interface{}) error {
	switch choice {
	// TODO implement other service choices
	case types.SERVICE_UNCONFIRMED_I_AM:
		req := IAmServiceRequest(request)
		if ok := dest.(*Device); ok != nil {
			return req.Decode(ok)
		} else {
			return errors.New("Invalid dest type passed")
		}

	default:
		panic("Unsupported service choice")
	}
}
