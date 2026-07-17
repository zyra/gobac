// Package responder dispatches incoming BACnet/IP service requests and writes
// handler responses over a transport.Conn.
package responder

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// Handler processes a decoded BACnet request. Returning no responses is valid;
// returning multiple responses is useful for discovery services.
//
// The request is released after Handler returns and must not be retained.
type Handler func(context.Context, *Request) ([]Response, error)

// Request contains a decoded BACnet packet and its transport addressing.
type Request struct {
	Packet      *bacnet.Request
	Source      transport.Endpoint
	Destination transport.Endpoint
}

type route struct {
	pduType types.PduType
	service uint8
}

// Server dispatches APDUs to registered service handlers.
type Server struct {
	mu       sync.RWMutex
	handlers map[route]Handler

	// ErrorHandler receives malformed-packet, handler, encoding, and write
	// errors encountered by Serve. ServeDatagram returns these errors directly.
	ErrorHandler func(error)
}

// NewServer creates an empty responder.
func NewServer() *Server {
	return &Server{handlers: make(map[route]Handler)}
}

// Handle registers a handler for a PDU type and service choice. Passing a nil
// handler removes the route.
func (s *Server) Handle(pduType types.PduType, serviceChoice uint8, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handlers == nil {
		s.handlers = make(map[route]Handler)
	}
	key := route{pduType: pduType & 0xf0, service: serviceChoice}
	if handler == nil {
		delete(s.handlers, key)
		return
	}
	s.handlers[key] = handler
}

// Serve reads and handles datagrams until the context is canceled or the
// transport fails. Packet-level errors are reported to ErrorHandler and do not
// stop the receive loop.
func (s *Server) Serve(ctx context.Context, conn transport.Conn) error {
	if conn == nil {
		return errors.New("responder: transport cannot be nil")
	}
	for {
		datagram, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if err := s.ServeDatagram(ctx, conn, datagram); err != nil {
			s.reportError(err)
		}
	}
}

// ServeDatagram parses, dispatches, and writes all responses for one datagram.
func (s *Server) ServeDatagram(ctx context.Context, conn transport.Conn, datagram transport.Datagram) error {
	if conn == nil {
		return errors.New("responder: transport cannot be nil")
	}
	packet, err := bacnet.ParseRequest(datagram.Payload, datagram.Source.UDPAddr())
	if err != nil {
		return fmt.Errorf("responder: decode packet: %v", err)
	}
	defer packet.Release()
	if int(packet.Header.BvlcLength) != len(datagram.Payload) {
		return fmt.Errorf("responder: BVLC length is %d, datagram length is %d", packet.Header.BvlcLength, len(datagram.Payload))
	}
	if err := validateRoutingSource(packet); err != nil {
		return err
	}
	if packet.Apdu.PduType == types.PduTypeConfirmedServiceRequest && packet.Apdu.Segmented {
		return s.writeResponse(ctx, conn, packet, datagram.Source,
			Abort(types.AbortReasonSegmentationNotSupported))
	}

	pduType := packet.Apdu.PduType & 0xf0
	handler := s.handler(pduType, packet.Apdu.ServiceChoice)
	if handler == nil {
		if pduType == types.PduTypeConfirmedServiceRequest {
			return s.writeResponse(ctx, conn, packet, datagram.Source,
				Reject(types.RejectReasonUnrecognizedService))
		}
		// Unknown unconfirmed services and response APDUs are deliberately
		// ignored, matching the service dispatcher behavior in bacnet-stack.
		return nil
	}

	responses, err := handler(ctx, &Request{
		Packet:      packet,
		Source:      cloneEndpoint(datagram.Source),
		Destination: cloneEndpoint(datagram.Destination),
	})
	if err != nil {
		return fmt.Errorf("responder: handler for PDU %#x service %d: %v", pduType, packet.Apdu.ServiceChoice, err)
	}
	for _, response := range responses {
		if err := s.writeResponse(ctx, conn, packet, datagram.Source, response); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handler(pduType types.PduType, serviceChoice uint8) Handler {
	s.mu.RLock()
	handler := s.handlers[route{pduType: pduType, service: serviceChoice}]
	s.mu.RUnlock()
	return handler
}

func (s *Server) writeResponse(ctx context.Context, conn transport.Conn, incoming *bacnet.Request, source transport.Endpoint, response Response) error {
	destination := cloneEndpoint(source)
	if response.Destination != nil {
		destination = cloneEndpoint(*response.Destination)
	}

	packet := bacnet.NewRequest()
	defer packet.Release()
	packet.Npci.Priority = incoming.Npci.Priority
	if incoming.Npci.SourceNet != 0 && incoming.Npci.SourceMAC != nil {
		mac := append((*incoming.Npci.SourceMAC)[:0:0], (*incoming.Npci.SourceMAC)...)
		packet.Npci.DestinationNet = incoming.Npci.SourceNet
		packet.Npci.DestinationLength = uint8(len(mac))
		packet.Npci.DestinationMAC = &mac
	}
	packet.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	if response.Broadcast {
		packet.Header.Function = types.BvlcFunctionOriginalBroadcastNpdu
		packet.Npci.DestinationNet = 0xffff
		packet.Npci.DestinationLength = 0
		packet.Npci.DestinationMAC = nil
	}

	pduType := response.PDUType & 0xf0
	packet.Apdu.PduType = pduType
	switch pduType {
	case types.PduTypeSimpleAck:
		if incoming.Apdu.PduType&0xf0 != types.PduTypeConfirmedServiceRequest {
			return errors.New("responder: SimpleACK requires a confirmed request")
		}
		if len(response.Payload) != 0 {
			return errors.New("responder: SimpleACK cannot contain a payload")
		}
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.ServiceChoice = incoming.Apdu.ServiceChoice
	case types.PduTypeComplexAck:
		if incoming.Apdu.PduType&0xf0 != types.PduTypeConfirmedServiceRequest {
			return errors.New("responder: ComplexACK requires a confirmed request")
		}
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.ServiceChoice = incoming.Apdu.ServiceChoice
		packet.Apdu.Payload = cloneBytes(response.Payload)
	case types.PduTypeError:
		if incoming.Apdu.PduType&0xf0 != types.PduTypeConfirmedServiceRequest {
			return errors.New("responder: Error APDU requires a confirmed request")
		}
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.ServiceChoice = incoming.Apdu.ServiceChoice
		packet.Apdu.ErrorClass = response.ErrorClass
		packet.Apdu.ErrorCode = response.ErrorCode
		// A non-empty payload replaces the generic errorClass/errorCode
		// encoding, e.g. the WritePropertyMultiple-Error production. See
		// pdu.Apdu.MarshalBinary's PduTypeError case.
		packet.Apdu.Payload = cloneBytes(response.Payload)
	case types.PduTypeReject:
		if incoming.Apdu.PduType&0xf0 != types.PduTypeConfirmedServiceRequest {
			return errors.New("responder: Reject requires a confirmed request")
		}
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.RejectReason = response.RejectReason
	case types.PduTypeAbort:
		if incoming.Apdu.PduType&0xf0 != types.PduTypeConfirmedServiceRequest {
			return errors.New("responder: Abort requires a confirmed request")
		}
		// Bit zero is the server flag in an Abort APDU. pdu.Apdu preserves
		// low flag bits while selecting the type using the high nibble.
		packet.Apdu.PduType = types.PduTypeAbort
		packet.Apdu.Server = true
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.AbortReason = response.AbortReason
	case types.PduTypeUnconfirmedServiceRequest:
		packet.Apdu.ServiceChoice = response.ServiceChoice
		packet.Apdu.Payload = cloneBytes(response.Payload)
	case types.PduTypeConfirmedServiceRequest:
		if response.InvokeID == 0 {
			return errors.New("responder: outgoing confirmed request requires a nonzero invoke ID")
		}
		packet.Npci.ExpectingReply = true
		packet.Apdu.InvokeID = response.InvokeID
		packet.Apdu.ServiceChoice = response.ServiceChoice
		packet.Apdu.Payload = cloneBytes(response.Payload)
	default:
		return fmt.Errorf("responder: unsupported response PDU type %#x", response.PDUType)
	}

	apdu, err := packet.Apdu.MarshalBinary()
	if err != nil {
		return fmt.Errorf("responder: encode response APDU: %v", err)
	}
	if pduType == types.PduTypeComplexAck && len(apdu) > int(incoming.Apdu.MaxApdu) {
		packet.Apdu.Reset()
		packet.Apdu.PduType = types.PduTypeAbort
		packet.Apdu.Server = true
		packet.Apdu.InvokeID = incoming.Apdu.InvokeID
		packet.Apdu.AbortReason = types.AbortReasonSegmentationNotSupported
		apdu, err = packet.Apdu.MarshalBinary()
		if err != nil {
			return fmt.Errorf("responder: encode fallback Abort APDU: %v", err)
		}
	}
	if len(apdu) > types.MaxApdu {
		return fmt.Errorf("responder: response APDU is %d octets, maximum is %d", len(apdu), types.MaxApdu)
	}

	encoded, err := packet.MarshalBinary()
	if err != nil {
		return fmt.Errorf("responder: encode response: %v", err)
	}
	if err := conn.Write(ctx, destination, encoded); err != nil {
		return fmt.Errorf("responder: write response: %v", err)
	}
	return nil
}

func validateRoutingSource(packet *bacnet.Request) error {
	macLength := 0
	if packet.Npci.SourceMAC != nil {
		macLength = len(*packet.Npci.SourceMAC)
	}
	if packet.Npci.SourceNet == 0 {
		if macLength != 0 || packet.Npci.SourceLength != 0 {
			return errors.New("responder: routed source address has no source network")
		}
		return nil
	}
	if macLength == 0 || packet.Npci.SourceLength == 0 {
		return errors.New("responder: routed source network has no source address")
	}
	if macLength != int(packet.Npci.SourceLength) {
		return errors.New("responder: routed source address length does not match source address")
	}
	if macLength > types.MaxMacLen {
		return fmt.Errorf("responder: routed source address exceeds %d octets", types.MaxMacLen)
	}
	return nil
}

func (s *Server) reportError(err error) {
	s.mu.RLock()
	handler := s.ErrorHandler
	s.mu.RUnlock()
	if handler != nil {
		handler(err)
	}
}
