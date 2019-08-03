package gobac

import (
	"fmt"
	"github.com/zyra/gobac/encoding"
	"net"
)

type Response struct {
	*Pdu
	Valid                bool
	Sender               *net.UDPAddr
	MaxSegments          uint32
	MaxAPDU              uint32
	SequenceNumber       uint32
	ProposedWindowNumber uint32
	IsBroadcast          bool
	Failed               bool
}

func NewResponse(data []byte) *Response {
	pdu := &Response{
		Pdu: NewPdu(),
	}

	pdu.Pdu.Buffer = encoding.NewBuffer(data)

	return pdu
}

func (r *Response) DecodeHeader() error {
	r.ProtocolType = r.NextOne()

	if r.ProtocolType != BACnetProtocol {
		return fmt.Errorf("expected protocol to be %x but got %x", BACnetProtocol, r.ProtocolType)
	}

	r.Function = r.NextOne()

	switch r.Function {
	case OriginalUnicastNPDU:
		break

	case OriginalBroadcastNPDU:
		r.IsBroadcast = true
		break

	default:
		return fmt.Errorf("received a function type that doesn't interest us: %x", r.Function)
	}

	r.BVLCLength = r.DecodeUnsigned16()
	r.NPDULength = r.BVLCLength - 4
	r.Truncate(int(r.NPDULength))

	return nil
}

func (r *Response) DecodeNPCI() error {
	l := r.Len()

	r.ProtocolVersion = r.NextOne()

	if r.ProtocolVersion != BACnetVersion {
		return fmt.Errorf("expected protocol version to be %d but got %d", 1, r.ProtocolVersion)
	}

	ctrl := r.NextOne()

	if r.NetworkLayerMessage = ctrl&0x80 != 0; r.NetworkLayerMessage {
		return fmt.Errorf("network layer messages aren't supported")
	}

	r.ControlOctet = ctrl

	if hasDest := ctrl & 0x20; hasDest != 0 {
		// DNET, DLEN, and DADR are present
		// We don't need this info since we're not dealing with raw packets
		// Let's just shave a few bytes off the buffer
		// DNET is 2 octets
		// DLEN is 1 octet
		// DADR is DLEN
		// +1 octet for hop count
		r.Next(2)                // dnet
		dLen := r.NextOne()      // dlen
		r.Next(int(dLen))        // dadr
		r.HopCount = r.NextOne() // hopcount
	}

	if hasSrc := ctrl & 0x08; hasSrc != 0 {
		// SNET, SLEN, SADR are present
		// Let's shave the bytes off
		// SNET = 2 octets
		// SLEN = 1 octet
		// SADR = SLEN
		r.Next(2)           // snet
		sLen := r.NextOne() // slen
		r.Next(int(sLen))   // sadr
	}

	// Expecting reply is
	r.ExpectingReply = ctrl&0x04 != 0

	// Priority is the first 2 bits
	r.Priority = ctrl & 0x03

	magicByte := r.NextOne()

	r.PduType = magicByte & 0xF0

	switch r.PduType {
	case PduTypeUnconfirmedServiceRequest:
		break

	case PduTypeSegmentAck:
		fmt.Println("Got a segment ack and I don't know how to handle this")
		fallthrough
	case PduTypeConfirmedServiceRequest:
		fmt.Println("shouldnt really get here..")
		break

	case PduTypeSimpleAck:
		bytes := r.Bytes()
		if len(bytes) == 2 {
			r.InvokeID = r.NextOne()
		}
		r.ServiceChoice = r.NextOne()
		break

	case PduTypeComplexAck:
		r.InvokeID = r.NextOne()

		if segmented := magicByte & 0x8; segmented != 0 {
			// Sequence number
			r.NextOne()

			// Proposed window size
			r.NextOne()

			panic("segmentation is not handled!")
		}

		break

	case PduTypeError, PduTypeReject, PduTypeAbort:
		r.InvokeID = r.NextOne()
		break

	default:
		return fmt.Errorf("unsupported pdu type: %x", r.PduType)
	}

	r.ServiceChoice = r.NextOne()
	r.MessageLength = r.NPDULength - uint16(l-r.Len())

	return nil
}

func (r *Response) Decode() error {
	if err := r.DecodeHeader(); err != nil {
		r.Valid = false
		return err
	}

	if err := r.DecodeNPCI(); err != nil {
		r.Valid = false
		return err
	}

	r.Valid = true

	return nil
}
