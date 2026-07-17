package bacnet

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet/types"
)

type failingMarshaler struct{}

func (failingMarshaler) MarshalBinary() ([]byte, error) {
	return nil, ErrInvokeIDExhausted
}

func resetTransactionsForTest() {
	mtx.Lock()
	transactions = make(map[string]bool)
	transactionNext = make(map[string]uint8)
	mtx.Unlock()
}

func newLifecycleTestServer() *Server {
	return &Server{
		networkSet: &networkSet{
			IPv4:          net.IPv4(127, 0, 0, 1),
			BroadcastIPv4: net.IPv4(127, 255, 255, 255),
		},
		DefaultTimeout: time.Second,
		close:          make(chan struct{}),
		start:          make(chan struct{}),
		cHandlersMtx:   &sync.RWMutex{},
		cHandlers:      make(map[string]confirmedHandler),
		ucHandlersMtx:  &sync.RWMutex{},
		ucHandlers:     make(map[types.UnconfirmedService]map[uint64]chan<- *Request),
		covHandlersMtx: &sync.RWMutex{},
		covHandlers:    make(map[string]chan<- *Request),
	}
}

func TestTryGetInvokeIDExhaustionAndReuse(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()
	address := net.IPv4(192, 0, 2, 10)
	seen := make(map[uint8]bool)

	for i := 0; i < 255; i++ {
		invokeID, err := TryGetInvokeID(address)
		if err != nil {
			t.Fatalf("reserve invoke ID %d: %v", i, err)
		}
		if invokeID == 0 || seen[invokeID] {
			t.Fatalf("invalid or duplicate invoke ID %d", invokeID)
		}
		seen[invokeID] = true
	}

	started := time.Now()
	invokeID, err := TryGetInvokeID(address)
	if err != ErrInvokeIDExhausted || invokeID != 0 {
		t.Fatalf("expected exhaustion, got ID %d and error %v", invokeID, err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("invoke ID exhaustion did not return promptly")
	}

	ReleaseInvokeID(address, 42)
	invokeID, err = TryGetInvokeID(address)
	if err != nil || invokeID != 42 {
		t.Fatalf("expected released ID 42, got ID %d and error %v", invokeID, err)
	}
}

func TestRequestReleaseCleansConfirmedTransaction(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 20)
	req := NewRequest()
	req.SetConfirmedService(types.ConfirmedServiceReadProperty, nil, address)
	invokeID := req.InvokeID()
	req.server = server
	server.SetConfirmedHandler(address, invokeID, req.tx)

	req.Release()

	if handler := server.getConfirmedHandler(address, invokeID); handler != nil {
		t.Fatal("confirmed handler remained registered after request release")
	}
	mtx.RLock()
	reserved := transactions[genTxId(address.String(), invokeID)]
	mtx.RUnlock()
	if reserved {
		t.Fatal("invoke ID remained reserved after request release")
	}
}

func TestRequestReleaseDrainsQueuedResponse(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 21)
	req := NewRequest()
	req.SetConfirmedService(types.ConfirmedServiceReadProperty, nil, address)
	req.server = server
	server.SetConfirmedHandler(address, req.InvokeID(), req.tx)
	response := NewRequest()
	if found, delivered := server.deliverConfirmedHandler(address, req.InvokeID(), response); !found || !delivered {
		t.Fatal("failed to queue response")
	}
	responseQueue := req.tx

	req.Release()
	if len(responseQueue) != 0 {
		t.Fatal("queued response remained after request release")
	}
}

func TestConfirmedHandlerIgnoresWrongService(t *testing.T) {
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 23)
	handler := make(chan *Request, 1)
	server.setConfirmedHandlerForService(address, 9, types.ConfirmedServiceReadProperty, handler)

	wrong := NewRequest()
	wrong.Apdu.PduType = types.PduTypeSimpleAck
	wrong.Apdu.InvokeID = 9
	wrong.Apdu.ServiceChoice = types.ConfirmedServiceWriteProperty
	if found, delivered := server.deliverConfirmedHandler(address, 9, wrong); found || delivered {
		wrong.Release()
		t.Fatalf("wrong service delivery = found %v, delivered %v", found, delivered)
	}
	wrong.Release()
	if server.getConfirmedHandler(address, 9) == nil {
		t.Fatal("wrong service removed the confirmed handler")
	}

	correct := NewRequest()
	correct.Apdu.PduType = types.PduTypeComplexAck
	correct.Apdu.InvokeID = 9
	correct.Apdu.ServiceChoice = types.ConfirmedServiceReadProperty
	if found, delivered := server.deliverConfirmedHandler(address, 9, correct); !found || !delivered {
		correct.Release()
		t.Fatalf("correct service delivery = found %v, delivered %v", found, delivered)
	}
	(<-handler).Release()
}

func TestSendMarshalFailureCleansTransaction(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 22)
	req := NewRequest()
	req.SetConfirmedService(types.ConfirmedServiceReadProperty, failingMarshaler{}, address)
	invokeID := req.InvokeID()

	if err := req.Send(address, server); err == nil {
		t.Fatal("expected marshal failure")
	}
	mtx.RLock()
	reserved := transactions[genTxId(address.String(), invokeID)]
	mtx.RUnlock()
	if reserved {
		t.Fatal("invoke ID remained reserved after marshal failure")
	}
	req.Release()
}

func TestRemoveHandlersDeletesEntries(t *testing.T) {
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 30)
	server.SetConfirmedHandler(address, 7, make(chan *Request, 1))
	server.RemoveConfirmedHandler(address, 7)
	if len(server.cHandlers) != 0 {
		t.Fatalf("confirmed handler map contains %d stale entries", len(server.cHandlers))
	}

	server.SetCovHandler(address, 8, make(chan *Request, 1))
	server.RemoveCovHandler(address, 8)
	if len(server.covHandlers) != 0 {
		t.Fatalf("COV handler map contains %d stale entries", len(server.covHandlers))
	}
}

func TestCovHandlersPreserveUnsigned32ProcessIdentifier(t *testing.T) {
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 31)
	server.SetCovHandler(address, 1, make(chan *Request, 1))
	server.SetCovHandlerWithProcessID(address, 65537, make(chan *Request, 1))

	if server.getCovHandler(address, 1) == nil {
		t.Fatal("low process identifier handler is missing")
	}
	if server.getCovHandler(address, 65537) == nil {
		t.Fatal("32-bit process identifier handler is missing")
	}
	if len(server.covHandlers) != 2 {
		t.Fatalf("process identifier keys collided: got %d handlers", len(server.covHandlers))
	}
}

func TestCovHandlersAcceptZeroProcessIdentifier(t *testing.T) {
	server := newLifecycleTestServer()
	address := net.IPv4(192, 0, 2, 32)
	handler := make(chan *Request, 1)
	server.SetCovHandler(address, 0, handler)
	if server.getCovHandler(address, 0) == nil {
		t.Fatal("zero process identifier handler is missing")
	}
	request := NewRequest()
	if found, delivered := server.deliverCovHandler(address, 0, request); !found || !delivered {
		request.Release()
		t.Fatalf("zero process identifier delivery = found %v, delivered %v", found, delivered)
	}
	(<-handler).Release()
	server.RemoveCovHandler(address, 0)
	if len(server.covHandlers) != 0 {
		t.Fatal("zero process identifier handler was not removed")
	}
}

func TestDeliverRequestDoesNotBlockFullHandler(t *testing.T) {
	handler := make(chan *Request, 1)
	handler <- NewRequest()
	req := NewRequest()
	defer req.Release()

	if deliverRequest(handler, req) {
		t.Fatal("delivery unexpectedly succeeded to a full handler")
	}
	filled := <-handler
	filled.Release()
}

func TestUnconfirmedSubscriptionsAreIndependent(t *testing.T) {
	server := newLifecycleTestServer()
	first := NewRequest()
	second := NewRequest()
	service := types.UnconfirmedServiceIAm
	first.server = server
	first.unconfirmedService = service
	first.unconfirmedToken = server.addUnconfirmedHandler(service, make(chan *Request, 1))
	second.server = server
	second.unconfirmedService = service
	second.unconfirmedToken = server.addUnconfirmedHandler(service, make(chan *Request, 1))

	first.Release()
	if got := len(server.getUnconfirmedHandlers(service)); got != 1 {
		t.Fatalf("expected one remaining subscription, got %d", got)
	}
	second.Release()
	if got := len(server.getUnconfirmedHandlers(service)); got != 0 {
		t.Fatalf("expected all subscriptions removed, got %d", got)
	}
}

func TestParseRequestRejectsMissingSender(t *testing.T) {
	if req, err := ParseRequest([]byte{0x81, 0x0a, 0x00, 0x06, 0x01, 0x00}, nil); err == nil || req != nil {
		t.Fatalf("expected missing sender error, got request %v and error %v", req, err)
	}
}

func TestRequestResetClearsAPDUState(t *testing.T) {
	req := NewRequest()
	req.Apdu.MoreFollows = true
	req.Apdu.ErrorClass = 9
	req.Apdu.AbortReason = 4
	req.Release()

	reused := NewRequest()
	defer reused.Release()
	if reused.Apdu.MoreFollows || reused.Apdu.ErrorClass != 0 || reused.Apdu.AbortReason != 0 {
		t.Fatal("pooled request retained APDU state")
	}
}

func TestChangingRequestServiceReleasesPreviousInvokeID(t *testing.T) {
	resetTransactionsForTest()
	defer resetTransactionsForTest()
	address := net.IPv4(192, 0, 2, 40)
	req := NewRequest()
	req.SetConfirmedService(types.ConfirmedServiceReadProperty, nil, address)
	invokeID := req.InvokeID()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoIs, nil)
	defer req.Release()

	if req.InvokeID() != 0 {
		t.Fatalf("unconfirmed request retained invoke ID %d", req.InvokeID())
	}
	mtx.RLock()
	reserved := transactions[genTxId(address.String(), invokeID)]
	mtx.RUnlock()
	if reserved {
		t.Fatal("previous invoke ID remained reserved after changing service")
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	server := newLifecycleTestServer()
	server.Shutdown()
	server.Shutdown()

	select {
	case <-server.Close():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not close completion channel")
	}
}

func TestListenContextStartsAndStops(t *testing.T) {
	server := newLifecycleTestServer()
	server.BroadcastPort = 0
	server.ServerPort = 0
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.ListenContext(ctx) }()

	select {
	case <-server.Start():
	case <-time.After(time.Second):
		t.Fatal("listener did not start")
	}
	cancel()

	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not stop")
	}
	server.Shutdown()
}

func TestListenContextRejectsSecondStartWithoutStoppingFirst(t *testing.T) {
	server := newLifecycleTestServer()
	server.BroadcastPort = 0
	server.ServerPort = 0
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.ListenContext(ctx) }()
	select {
	case <-server.Start():
	case <-time.After(time.Second):
		t.Fatal("listener did not start")
	}

	if err := server.ListenContext(context.Background()); err != ErrServerAlreadyListening {
		t.Fatalf("expected already-listening error, got %v", err)
	}
	select {
	case <-server.Close():
		t.Fatal("second start stopped the active listener")
	default:
	}

	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not stop")
	}
}

func TestListenerReservationRollsBack(t *testing.T) {
	server := newLifecycleTestServer()
	if err := server.reserveListener(); err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	if err := server.reserveListener(); err != ErrServerAlreadyListening {
		t.Fatalf("expected already-listening error, got %v", err)
	}
	server.cancelListenerStart()
	if err := server.reserveListener(); err != nil {
		t.Fatalf("reserve listener after rollback: %v", err)
	}
	server.cancelListenerStart()
}

func TestListenAndShutdownConcurrentStartup(t *testing.T) {
	for i := 0; i < 25; i++ {
		server := newLifecycleTestServer()
		server.BroadcastPort = 0
		server.ServerPort = 0
		result := make(chan error, 1)
		go func() { result <- server.ListenContext(context.Background()) }()
		server.Shutdown()

		select {
		case <-result:
		case <-time.After(time.Second):
			t.Fatal("listener did not stop during concurrent startup")
		}
	}
}

func TestListenWrapperSignalsStoppedAfterStartupFailure(t *testing.T) {
	server := newLifecycleTestServer()
	server.Shutdown()
	server.Listen(context.Background())

	select {
	case <-server.Start():
	default:
		t.Fatal("start channel was not closed after startup failure")
	}
	select {
	case <-server.Close():
	default:
		t.Fatal("close channel was not closed after startup failure")
	}
}

func TestSendBeforeListenReturnsError(t *testing.T) {
	server := newLifecycleTestServer()
	if err := server.Send([]byte{1}, &net.UDPAddr{}); err == nil {
		t.Fatal("expected send-before-listen error")
	}
}

func TestSendConcurrentWithStartup(t *testing.T) {
	server := newLifecycleTestServer()
	server.BroadcastPort = 0
	server.ServerPort = 0
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.ListenContext(ctx) }()
	for i := 0; i < 100; i++ {
		_ = server.Send([]byte{1}, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	}
	cancel()
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("listener did not stop")
	}
}

func TestSubscribeCovCancellationPayload(t *testing.T) {
	const processID uint32 = 0x01020304
	cancelPayload := newSubscribeCovPayload(types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3}, processID, true)
	activePayload := newSubscribeCovPayload(types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3}, processID, false)
	if !cancelPayload.Cancel {
		t.Fatal("cancellation flag was not copied to SubscribeCOV payload")
	}
	if cancelPayload.ProcessIdentifier32 != processID {
		t.Fatalf("process identifier truncated: got %#x", cancelPayload.ProcessIdentifier32)
	}
	cancelBytes, err := cancelPayload.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal cancellation: %v", err)
	}
	activeBytes, err := activePayload.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal subscription: %v", err)
	}
	if len(cancelBytes) >= len(activeBytes) {
		t.Fatalf("cancellation encoded optional subscription fields: cancel=%x active=%x", cancelBytes, activeBytes)
	}
}

// scriptedUDPReader is a udpReader test double whose ReadFromUDP behavior is
// driven by a caller-supplied function keyed on call number, so receive()'s
// error handling can be exercised without a real socket.
type scriptedUDPReader struct {
	mtx   sync.Mutex
	calls int
	fn    func(call int) (int, *net.UDPAddr, error)
}

func (r *scriptedUDPReader) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	r.mtx.Lock()
	r.calls++
	call := r.calls
	r.mtx.Unlock()
	return r.fn(call)
}

func TestReceiveSurvivesTransientReadError(t *testing.T) {
	server := newLifecycleTestServer()
	server.rcvErrors = true
	server.error = make(chan error, 50)

	callNotify := make(chan int, 10)
	unblock := make(chan struct{})
	reader := &scriptedUDPReader{}
	reader.fn = func(call int) (int, *net.UDPAddr, error) {
		callNotify <- call
		switch call {
		case 1:
			return 0, nil, errors.New("read udp4: no buffer space available")
		case 2:
			return 0, nil, errors.New("read udp4: no buffer space available (2)")
		default:
			<-unblock
			return 0, nil, errors.New("use of closed network connection")
		}
	}

	done := make(chan struct{})
	go func() {
		server.receive(reader, context.Background())
		close(done)
	}()

	// The reader's call count must reach at least 3 within 1s -- proving the
	// loop survived the first two transient errors instead of returning
	// after the first one (the old, buggy behavior).
	timeout := time.After(time.Second)
	for i := 1; i <= 3; i++ {
		select {
		case call := <-callNotify:
			if call != i {
				t.Fatalf("expected call %d to arrive next, got %d", i, call)
			}
		case <-timeout:
			t.Fatalf("call count did not reach 3 within 1s (observed %d calls)", i-1)
		}
	}

	// Both transient errors must have been reported.
	for i := 0; i < 2; i++ {
		select {
		case err := <-server.Errors():
			if err == nil {
				t.Fatal("expected non-nil transient error on Errors()")
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for transient error %d to be reported", i+1)
		}
	}

	// Trigger shutdown, then unblock the third (blocked) read so it can
	// return the fatal closed-connection error and the loop can exit.
	server.Shutdown()
	close(unblock)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("receive did not exit after shutdown")
	}
}

func TestReceiveStopsOnClosedConnectionError(t *testing.T) {
	server := newLifecycleTestServer()

	reader := &scriptedUDPReader{
		fn: func(call int) (int, *net.UDPAddr, error) {
			return 0, nil, errors.New("use of closed network connection")
		},
	}

	done := make(chan struct{})
	go func() {
		server.receive(reader, context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("receive did not exit on closed-connection error")
	}

	reader.mtx.Lock()
	calls := reader.calls
	reader.mtx.Unlock()
	if calls != 1 {
		t.Fatalf("expected exactly 1 read call, got %d (possible hot spin)", calls)
	}
}

func TestReceiveStopsWhenContextCanceled(t *testing.T) {
	server := newLifecycleTestServer()
	ctx, cancel := context.WithCancel(context.Background())

	// firstCall is closed exactly once, on the first read, without blocking
	// subsequent (possibly many, since this is a tight retry loop) reads --
	// an unbounded/blocking notification channel here could deadlock the
	// receive goroutine before it ever gets to observe cancellation.
	firstCall := make(chan struct{})
	reader := &scriptedUDPReader{
		fn: func(call int) (int, *net.UDPAddr, error) {
			if call == 1 {
				close(firstCall)
			}
			return 0, nil, errors.New("read udp4: no buffer space available")
		},
	}

	done := make(chan struct{})
	go func() {
		server.receive(reader, ctx)
		close(done)
	}()

	select {
	case <-firstCall:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first read call")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("receive did not exit after context cancellation")
	}
}
