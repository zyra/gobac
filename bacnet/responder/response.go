package responder

import (
	"github.com/zyra/gobac/bacnet/transport"
	"github.com/zyra/gobac/bacnet/types"
)

// Response describes one APDU to return from a Handler. Confirmed response
// types inherit the invoke ID and service choice from the incoming request.
type Response struct {
	PDUType       types.PduType
	ServiceChoice uint8
	Payload       []byte
	ErrorClass    types.ErrorClass
	ErrorCode     types.ErrorCode
	RejectReason  types.RejectReason
	AbortReason   types.AbortReason
	InvokeID      uint8

	Destination *transport.Endpoint
	Broadcast   bool
}

// SimpleACK acknowledges a confirmed service without service parameters.
func SimpleACK() Response {
	return Response{PDUType: types.PduTypeSimpleAck}
}

// ComplexACK acknowledges a confirmed service and includes service parameters.
func ComplexACK(payload []byte) Response {
	return Response{PDUType: types.PduTypeComplexAck, Payload: cloneBytes(payload)}
}

// ErrorResponse returns the standard BACnet error-class and error-code pair.
func ErrorResponse(class types.ErrorClass, code types.ErrorCode) Response {
	return Response{PDUType: types.PduTypeError, ErrorClass: class, ErrorCode: code}
}

// Reject rejects a confirmed request for the supplied protocol reason.
func Reject(reason types.RejectReason) Response {
	return Response{PDUType: types.PduTypeReject, RejectReason: reason}
}

// Abort aborts a confirmed transaction. Abort responses are encoded with the
// server flag set because they originate from this responder.
func Abort(reason types.AbortReason) Response {
	return Response{PDUType: types.PduTypeAbort, AbortReason: reason}
}

// Unconfirmed creates an unconfirmed service request, such as an I-Am reply to
// Who-Is. The service choice is independent from the incoming request.
func Unconfirmed(serviceChoice uint8, payload []byte) Response {
	return Response{
		PDUType:       types.PduTypeUnconfirmedServiceRequest,
		ServiceChoice: serviceChoice,
		Payload:       cloneBytes(payload),
	}
}

// Confirmed creates an outgoing confirmed service request. Transaction retry
// and timeout policy remain the caller's responsibility.
func Confirmed(serviceChoice, invokeID uint8, payload []byte) Response {
	return Response{
		PDUType:       types.PduTypeConfirmedServiceRequest,
		ServiceChoice: serviceChoice,
		InvokeID:      invokeID,
		Payload:       cloneBytes(payload),
	}
}

// To directs this response to an endpoint other than the request source.
func (r Response) To(endpoint transport.Endpoint) Response {
	endpoint = cloneEndpoint(endpoint)
	r.Destination = &endpoint
	return r
}

// AsBroadcast encodes the response as an Original-Broadcast-NPDU. The response
// destination still needs to be a broadcast endpoint understood by the
// transport in use.
func (r Response) AsBroadcast() Response {
	r.Broadcast = true
	return r
}

func cloneBytes(value []byte) []byte {
	return append([]byte(nil), value...)
}

func cloneEndpoint(endpoint transport.Endpoint) transport.Endpoint {
	endpoint.IP = append([]byte(nil), endpoint.IP...)
	endpoint.MAC = append([]byte(nil), endpoint.MAC...)
	return endpoint
}
