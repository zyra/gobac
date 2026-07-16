package responder

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/bacnet/types"
)

const testService = uint8(99)

type writtenDatagram struct {
	destination transport.Endpoint
	payload     []byte
}

type recordingConn struct {
	local  transport.Endpoint
	mu     sync.Mutex
	writes []writtenDatagram
	err    error
}

func (c *recordingConn) Read(ctx context.Context) (transport.Datagram, error) {
	<-ctx.Done()
	return transport.Datagram{}, ctx.Err()
}

func (c *recordingConn) Write(ctx context.Context, destination transport.Endpoint, payload []byte) error {
	if c.err != nil {
		return c.err
	}
	c.mu.Lock()
	c.writes = append(c.writes, writtenDatagram{destination: cloneEndpoint(destination), payload: cloneBytes(payload)})
	c.mu.Unlock()
	return nil
}

func (c *recordingConn) LocalEndpoint() transport.Endpoint { return cloneEndpoint(c.local) }
func (c *recordingConn) Close() error                      { return nil }

func (c *recordingConn) written() []writtenDatagram {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]writtenDatagram, len(c.writes))
	copy(result, c.writes)
	return result
}

func makePacket(t *testing.T, pduType types.PduType, service, invoke uint8, payload []byte) []byte {
	t.Helper()
	packet := bacnet.NewRequest()
	defer packet.Release()
	packet.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	packet.Apdu.PduType = pduType
	packet.Apdu.ServiceChoice = service
	packet.Apdu.InvokeID = invoke
	packet.Apdu.Payload = cloneBytes(payload)
	if pduType == types.PduTypeConfirmedServiceRequest {
		packet.Npci.ExpectingReply = true
	}
	encoded, err := packet.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func incomingDatagram(payload []byte) transport.Datagram {
	return transport.Datagram{
		Payload:     payload,
		Source:      transport.NewEndpoint(net.IPv4(192, 0, 2, 10), 47808),
		Destination: transport.NewEndpoint(net.IPv4(192, 0, 2, 20), 47808),
	}
}

func apduBytes(t *testing.T, packet []byte) []byte {
	t.Helper()
	if len(packet) < 6 {
		t.Fatalf("encoded packet has only %d bytes", len(packet))
	}
	return packet[6:]
}

func TestResponseEncoding(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantAPDU []byte
	}{
		{name: "simple ack", response: SimpleACK(), wantAPDU: []byte{0x20, 37, testService}},
		{name: "complex ack", response: ComplexACK([]byte{0xaa, 0xbb}), wantAPDU: []byte{0x30, 37, testService, 0xaa, 0xbb}},
		{name: "error", response: ErrorResponse(types.ErrorClassProperty, types.ErrorCodeUnknownProperty), wantAPDU: []byte{0x50, 37, testService, 0x91, byte(types.ErrorClassProperty), 0x91, byte(types.ErrorCodeUnknownProperty)}},
		{name: "reject", response: Reject(types.RejectReasonInvalidTag), wantAPDU: []byte{0x60, 37, byte(types.RejectReasonInvalidTag)}},
		{name: "server abort", response: Abort(types.AbortReasonSegmentationNotSupported), wantAPDU: []byte{0x71, 37, byte(types.AbortReasonSegmentationNotSupported)}},
		{name: "unconfirmed", response: Unconfirmed(types.UnconfirmedServiceIAm, []byte{1, 2, 3}), wantAPDU: []byte{0x10, byte(types.UnconfirmedServiceIAm), 1, 2, 3}},
		{name: "outgoing confirmed", response: Confirmed(types.ConfirmedServiceCovNotification, 88, []byte{1, 2}), wantAPDU: []byte{0x00, 0x05, 88, byte(types.ConfirmedServiceCovNotification), 1, 2}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer()
			server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
				return []Response{test.response}, nil
			})
			conn := &recordingConn{}
			request := makePacket(t, types.PduTypeConfirmedServiceRequest, testService, 37, []byte{9})
			if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(request)); err != nil {
				t.Fatal(err)
			}
			writes := conn.written()
			if len(writes) != 1 {
				t.Fatalf("got %d writes, want 1", len(writes))
			}
			got := apduBytes(t, writes[0].payload)
			if string(got) != string(test.wantAPDU) {
				t.Fatalf("APDU = % x, want % x", got, test.wantAPDU)
			}
			if !writes[0].destination.IP.Equal(incomingDatagram(nil).Source.IP) || writes[0].destination.Port != 47808 {
				t.Fatalf("destination = %v, want request source", writes[0].destination)
			}
		})
	}
}

func TestConfirmedResponsePreservesPriorityInvokeIDAndService(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(_ context.Context, request *Request) ([]Response, error) {
		if request.Packet.Apdu.InvokeID != 201 || request.Packet.Apdu.ServiceChoice != testService {
			t.Fatalf("handler received invoke/service %d/%d", request.Packet.Apdu.InvokeID, request.Packet.Apdu.ServiceChoice)
		}
		if !request.Source.IP.Equal(net.IPv4(192, 0, 2, 10)) {
			t.Fatalf("source = %v", request.Source)
		}
		return []Response{ComplexACK([]byte{0xde, 0xad})}, nil
	})
	packet := bacnet.NewRequest()
	packet.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	packet.Npci.ExpectingReply = true
	packet.Npci.Priority = types.MessagePriorityUrgent
	packet.Apdu.PduType = types.PduTypeConfirmedServiceRequest
	packet.Apdu.InvokeID = 201
	packet.Apdu.ServiceChoice = testService
	encoded, err := packet.MarshalBinary()
	packet.Release()
	if err != nil {
		t.Fatal(err)
	}

	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(encoded)); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if got := apduBytes(t, writes[0].payload); string(got) != string([]byte{0x30, 201, testService, 0xde, 0xad}) {
		t.Fatalf("APDU = % x", got)
	}
	// The NPDU control octet's two low bits carry message priority.
	if writes[0].payload[5]&0x03 != byte(types.MessagePriorityUrgent) {
		t.Fatalf("priority bits = %d, want %d", writes[0].payload[5]&0x03, types.MessagePriorityUrgent)
	}
}

func TestRoutedResponseTargetsOriginalSource(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
		return []Response{SimpleACK()}, nil
	})
	packet := bacnet.NewRequest()
	packet.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	packet.Npci.ExpectingReply = true
	packet.Npci.SourceNet = 123
	sourceMAC := net.HardwareAddr{0xaa, 0xbb, 0xcc}
	packet.Npci.SourceLength = uint8(len(sourceMAC))
	packet.Npci.SourceMAC = &sourceMAC
	packet.Apdu.PduType = types.PduTypeConfirmedServiceRequest
	packet.Apdu.InvokeID = 54
	packet.Apdu.ServiceChoice = testService
	encoded, err := packet.MarshalBinary()
	packet.Release()
	if err != nil {
		t.Fatal(err)
	}

	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(encoded)); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1", len(writes))
	}
	response, err := bacnet.ParseRequest(writes[0].payload, writes[0].destination.UDPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Release()
	if response.Npci.DestinationNet != 123 || response.Npci.DestinationMAC == nil ||
		string(*response.Npci.DestinationMAC) != string(sourceMAC) {
		t.Fatalf("routed destination = network %d, MAC %v", response.Npci.DestinationNet, response.Npci.DestinationMAC)
	}
	if response.Npci.SourceNet != 0 || response.Npci.SourceMAC != nil {
		t.Fatalf("response unexpectedly contains routed source: network %d, MAC %v", response.Npci.SourceNet, response.Npci.SourceMAC)
	}
	if response.Apdu.InvokeID != 54 || response.Apdu.ServiceChoice != testService || response.Apdu.PduType != types.PduTypeSimpleAck {
		t.Fatalf("response APDU = %#v", response.Apdu)
	}
}

func TestComplexACKHonorsRequesterMaximumAPDU(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantAPDU []byte
	}{
		{
			name:     "exact boundary",
			payload:  make([]byte, 47),
			wantAPDU: append([]byte{0x30, 91, testService}, make([]byte, 47)...),
		},
		{
			name:     "oversized aborts",
			payload:  make([]byte, 48),
			wantAPDU: []byte{0x71, 91, byte(types.AbortReasonSegmentationNotSupported)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer()
			server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
				return []Response{ComplexACK(test.payload)}, nil
			})
			request := bacnet.NewRequest()
			request.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
			request.Npci.ExpectingReply = true
			request.Apdu.PduType = types.PduTypeConfirmedServiceRequest
			request.Apdu.MaxApdu = 50
			request.Apdu.InvokeID = 91
			request.Apdu.ServiceChoice = testService
			encoded, err := request.MarshalBinary()
			request.Release()
			if err != nil {
				t.Fatal(err)
			}
			conn := &recordingConn{}
			if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(encoded)); err != nil {
				t.Fatal(err)
			}
			writes := conn.written()
			if len(writes) != 1 {
				t.Fatalf("got %d writes, want 1", len(writes))
			}
			if got := apduBytes(t, writes[0].payload); string(got) != string(test.wantAPDU) {
				t.Fatalf("APDU = % x, want % x", got, test.wantAPDU)
			}
		})
	}
}

func TestHandlerCanReturnZeroOrMultipleResponses(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeUnconfirmedServiceRequest, 70, func(context.Context, *Request) ([]Response, error) {
		return nil, nil
	})
	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(makePacket(t, types.PduTypeUnconfirmedServiceRequest, 70, 0, nil))); err != nil {
		t.Fatal(err)
	}
	if len(conn.written()) != 0 {
		t.Fatal("zero-response handler wrote a packet")
	}

	server.Handle(types.PduTypeUnconfirmedServiceRequest, 71, func(context.Context, *Request) ([]Response, error) {
		return []Response{
			Unconfirmed(72, []byte{1}),
			Unconfirmed(73, []byte{2}),
		}, nil
	})
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(makePacket(t, types.PduTypeUnconfirmedServiceRequest, 71, 0, nil))); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want 2", len(writes))
	}
	if string(apduBytes(t, writes[0].payload)) != string([]byte{0x10, 72, 1}) ||
		string(apduBytes(t, writes[1].payload)) != string([]byte{0x10, 73, 2}) {
		t.Fatalf("unexpected APDUs: % x, % x", apduBytes(t, writes[0].payload), apduBytes(t, writes[1].payload))
	}
}

func TestUnknownServices(t *testing.T) {
	server := NewServer()
	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(
		makePacket(t, types.PduTypeConfirmedServiceRequest, 222, 41, nil))); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 1 {
		t.Fatalf("confirmed unknown service produced %d writes", len(writes))
	}
	if got := apduBytes(t, writes[0].payload); string(got) != string([]byte{0x60, 41, byte(types.RejectReasonUnrecognizedService)}) {
		t.Fatalf("reject APDU = % x", got)
	}

	conn = &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(
		makePacket(t, types.PduTypeUnconfirmedServiceRequest, 222, 0, nil))); err != nil {
		t.Fatal(err)
	}
	if len(conn.written()) != 0 {
		t.Fatal("unknown unconfirmed service should be ignored")
	}
}

func TestSegmentedConfirmedRequestIsAborted(t *testing.T) {
	server := NewServer()
	conn := &recordingConn{}
	packet := []byte{
		0x81, 0x0a, 0x00, 0x0d,
		0x01, 0x04,
		0x0c, 0x05, 37, 0, 1, byte(types.ConfirmedServiceReadProperty), 0xaa,
	}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(packet)); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 1 {
		t.Fatalf("got %d writes", len(writes))
	}
	if got := apduBytes(t, writes[0].payload); string(got) != string([]byte{0x71, 37, byte(types.AbortReasonSegmentationNotSupported)}) {
		t.Fatalf("abort APDU = %x", got)
	}
}

func TestMalformedDatagramsAreRejected(t *testing.T) {
	valid := makePacket(t, types.PduTypeUnconfirmedServiceRequest, 222, 0, nil)
	wrongLength := append(cloneBytes(valid), 0)
	tests := []struct {
		name    string
		payload []byte
		match   string
	}{
		{name: "short", payload: []byte{0x81}, match: "too short"},
		{name: "wrong protocol", payload: []byte{0x82, 0x0a, 0, 6, 1, 0}, match: "expected protocol"},
		{name: "length mismatch", payload: wrongLength, match: "BVLC length"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := &recordingConn{}
			err := NewServer().ServeDatagram(context.Background(), conn, incomingDatagram(test.payload))
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want containing %q", err, test.match)
			}
			if len(conn.written()) != 0 {
				t.Fatal("malformed request produced a response")
			}
		})
	}
}

func TestHandlerAndWriteErrorsAreReturned(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
		return nil, errors.New("handler failed")
	})
	err := server.ServeDatagram(context.Background(), &recordingConn{}, incomingDatagram(
		makePacket(t, types.PduTypeConfirmedServiceRequest, testService, 1, nil)))
	if err == nil || !strings.Contains(err.Error(), "handler failed") {
		t.Fatalf("handler error = %v", err)
	}

	server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
		return []Response{SimpleACK()}, nil
	})
	err = server.ServeDatagram(context.Background(), &recordingConn{err: errors.New("write failed")}, incomingDatagram(
		makePacket(t, types.PduTypeConfirmedServiceRequest, testService, 1, nil)))
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("write error = %v", err)
	}
}

func TestInvalidHandlerResponseIsNotWritten(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeUnconfirmedServiceRequest, 74, func(context.Context, *Request) ([]Response, error) {
		return []Response{SimpleACK()}, nil
	})
	conn := &recordingConn{}
	err := server.ServeDatagram(context.Background(), conn, incomingDatagram(
		makePacket(t, types.PduTypeUnconfirmedServiceRequest, 74, 0, nil)))
	if err == nil || !strings.Contains(err.Error(), "requires a confirmed request") {
		t.Fatalf("error = %v", err)
	}
	if len(conn.written()) != 0 {
		t.Fatal("invalid response was written")
	}
}

func TestUnconfirmedResponseMaximumAPDU(t *testing.T) {
	tests := []struct {
		name        string
		payloadSize int
		wantError   bool
	}{
		{name: "exact boundary", payloadSize: types.MaxApdu - 2},
		{name: "one octet over", payloadSize: types.MaxApdu - 1, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer()
			server.Handle(types.PduTypeUnconfirmedServiceRequest, 75, func(context.Context, *Request) ([]Response, error) {
				return []Response{Unconfirmed(76, make([]byte, test.payloadSize))}, nil
			})
			conn := &recordingConn{}
			err := server.ServeDatagram(context.Background(), conn, incomingDatagram(
				makePacket(t, types.PduTypeUnconfirmedServiceRequest, 75, 0, nil)))
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "maximum") {
					t.Fatalf("error = %v", err)
				}
				if len(conn.written()) != 0 {
					t.Fatal("oversized response was written")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			writes := conn.written()
			if len(writes) != 1 || len(apduBytes(t, writes[0].payload)) != types.MaxApdu {
				t.Fatalf("boundary response writes = %d", len(writes))
			}
		})
	}
}

func TestMemoryTransportServeContinuesAfterMalformedDatagram(t *testing.T) {
	network := transport.NewMemoryNetwork()
	client, err := network.Listen(transport.NewEndpoint(net.IPv4(192, 0, 2, 30), 47808))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	deviceEndpoint := transport.NewEndpoint(net.IPv4(192, 0, 2, 31), 47808)
	device, err := network.Listen(deviceEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()

	server := NewServer()
	errorsSeen := make(chan error, 1)
	server.ErrorHandler = func(err error) { errorsSeen <- err }
	server.Handle(types.PduTypeConfirmedServiceRequest, testService, func(context.Context, *Request) ([]Response, error) {
		return []Response{SimpleACK()}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx, device) }()

	if err := client.Write(ctx, deviceEndpoint, []byte{0x81}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errorsSeen:
		if !strings.Contains(err.Error(), "too short") {
			t.Fatalf("malformed error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("malformed datagram was not reported")
	}

	request := makePacket(t, types.PduTypeConfirmedServiceRequest, testService, 77, nil)
	if err := client.Write(ctx, deviceEndpoint, request); err != nil {
		t.Fatal(err)
	}
	response, err := client.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := apduBytes(t, response.Payload); string(got) != string([]byte{0x20, 77, testService}) {
		t.Fatalf("response APDU = % x", got)
	}

	cancel()
	select {
	case err := <-serveDone:
		if err != context.Canceled {
			t.Fatalf("Serve error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop after cancellation")
	}
}

func TestBroadcastAndDestinationOverride(t *testing.T) {
	server := NewServer()
	destination := transport.NewEndpoint(net.IPv4bcast, 47809)
	server.Handle(types.PduTypeUnconfirmedServiceRequest, 80, func(context.Context, *Request) ([]Response, error) {
		return []Response{Unconfirmed(81, nil).To(destination).AsBroadcast()}, nil
	})
	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(
		makePacket(t, types.PduTypeUnconfirmedServiceRequest, 80, 0, nil))); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 1 || !writes[0].destination.IP.Equal(net.IPv4bcast) || writes[0].destination.Port != 47809 {
		t.Fatalf("destination = %v", writes)
	}
	if writes[0].payload[1] != byte(types.BvlcFunctionOriginalBroadcastNpdu) {
		t.Fatalf("BVLC function = %#x", writes[0].payload[1])
	}
	packet, err := bacnet.ParseRequest(writes[0].payload, writes[0].destination.UDPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer packet.Release()
	if packet.Npci.DestinationNet != 0xffff || packet.Npci.DestinationLength != 0 || packet.Npci.HopCount != 0xff {
		t.Fatalf("broadcast NPCI = %+v", packet.Npci)
	}
}

func TestBroadcastResponseClearsRoutedDestination(t *testing.T) {
	server := NewServer()
	server.Handle(types.PduTypeUnconfirmedServiceRequest, 80, func(context.Context, *Request) ([]Response, error) {
		return []Response{Unconfirmed(81, nil).AsBroadcast()}, nil
	})
	request := bacnet.NewRequest()
	request.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	request.Npci.SourceNet = 123
	sourceMAC := net.HardwareAddr{0xaa, 0xbb}
	request.Npci.SourceLength = uint8(len(sourceMAC))
	request.Npci.SourceMAC = &sourceMAC
	request.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	request.Apdu.ServiceChoice = 80
	encoded, err := request.MarshalBinary()
	request.Release()
	if err != nil {
		t.Fatal(err)
	}
	conn := &recordingConn{}
	if err := server.ServeDatagram(context.Background(), conn, incomingDatagram(encoded)); err != nil {
		t.Fatal(err)
	}
	writes := conn.written()
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1", len(writes))
	}
	response, err := bacnet.ParseRequest(writes[0].payload, writes[0].destination.UDPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Release()
	if response.Npci.DestinationNet != 0xffff || response.Npci.DestinationLength != 0 || response.Npci.DestinationMAC != nil {
		t.Fatalf("broadcast response retained routed destination: %+v", response.Npci)
	}
}
