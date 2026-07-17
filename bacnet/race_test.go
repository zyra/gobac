package bacnet

import (
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// buildSimpleAckBytes marshals a minimal confirmed-service SimpleAck frame
// carrying the given invoke ID, as if it had just arrived from a remote
// BACnet device on the wire.
func buildSimpleAckBytes(t *testing.T, invokeID uint8) []byte {
	t.Helper()
	req := NewRequest()
	req.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	req.Apdu.PduType = types.PduTypeSimpleAck
	req.Apdu.ServiceChoice = types.ConfirmedServiceWriteProperty
	req.Apdu.InvokeID = invokeID
	data, err := req.MarshalBinary()
	req.Release()
	if err != nil {
		t.Fatalf("marshal simple ack: %v", err)
	}
	return data
}

// TestConfirmedDispatchDoesNotRaceRequestAfterDelivery exercises the real
// receive/dispatch path (Server.handle) with a consumer goroutine that
// immediately Release()s the delivered *Request back into the sync.Pool,
// the same way client goroutines such as ReadProperty do. If the dispatch
// path reads req (or req.Apdu) after handing ownership of req to the
// consumer via the channel, the concurrent Reset() inside Release() races
// with that read under `go test -race`.
func TestConfirmedDispatchDoesNotRaceRequestAfterDelivery(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()

	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 77)
	udpAddr := &net.UDPAddr{IP: address, Port: 47808}

	const iterations = 200

	for i := 0; i < iterations; i++ {
		invokeID, err := TryGetInvokeID(address)
		if err != nil {
			t.Fatalf("iteration %d: reserve invoke id: %v", i, err)
		}

		handler := make(chan *Request, 1)
		server.SetConfirmedHandler(address, invokeID, handler)

		data := buildSimpleAckBytes(t, invokeID)

		done := make(chan struct{})
		go func() {
			// Mirrors the client side: take ownership of the delivered
			// request and immediately release it back to the pool.
			req := <-handler
			req.Release()
			close(done)
		}()

		server.handle(data, len(data), udpAddr)

		<-done
	}
}
