package bacnet

import (
	"net"
	"sync"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// TestUnmarshalBinaryDoesNotRaceScratchBuffer hammers Request.UnmarshalBinary
// from many goroutines concurrently. UnmarshalBinary borrows a scratch
// *bytes.Buffer from buffPool, decodes into it, and must return it to the
// pool only after it is done touching it. If the buffer is returned to the
// pool before it is reset, a concurrent Get() can hand the same buffer to
// another goroutine while the original goroutine is still resetting it,
// which `go test -race` detects as a data race.
func TestUnmarshalBinaryDoesNotRaceScratchBuffer(t *testing.T) {
	// Build valid, well-formed request bytes once up front.
	built := NewRequest()
	built.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	built.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	built.Apdu.ServiceChoice = types.UnconfirmedServiceWhoIs
	data, err := built.MarshalBinary()
	built.Release()
	if err != nil {
		t.Fatalf("marshal seed request: %v", err)
	}

	const goroutines = 8
	const iterations = 200

	sender := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 47808}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				req := NewRequest()
				req.Sender = sender
				if err := req.UnmarshalBinary(data); err != nil {
					req.Release()
					t.Errorf("unmarshal: %v", err)
					return
				}
				req.Release()
			}
		}()
	}
	wg.Wait()
}
