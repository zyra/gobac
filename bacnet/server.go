package bacnet

import (
	"context"
	"errors"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	ErrServerAlreadyListening = errors.New("server is already listening")
	ErrServerShutdown         = errors.New("server is shut down")
)

// The main server object.
// This object will allow you to Send requests and receive broadcasts/responses.
type Server struct {
	*networkSet
	ServerAddr     *net.UDPAddr
	ServerPort     uint16
	BroadcastAddr  *net.UDPAddr
	BroadcastPort  uint16
	UnicastConn    *net.UDPConn // Unicast
	BroadcastConn  *net.UDPConn // Broadcast
	DefaultTimeout time.Duration

	close     chan struct{}
	closing   bool
	listening bool
	stateMtx  sync.RWMutex
	startOnce sync.Once
	closeOnce sync.Once

	rcvErrors bool
	error     chan error

	start chan struct{}

	cHandlersMtx   *sync.RWMutex
	cHandlers      map[string]chan<- *Request
	ucHandlers     map[types.UnconfirmedService]map[uint64]chan<- *Request
	ucHandlersMtx  *sync.RWMutex
	ucHandlerID    uint64
	covHandlers    map[string]chan<- *Request
	covHandlersMtx *sync.RWMutex
}

var rxBuffPool = sync.Pool{
	New: func() interface{} {
		arr := new([types.MaxMpdu]byte)
		return arr[:]
	},
}

// Create a new server object with the provided configuration
func NewServer(config *ServerConfig) (*Server, error) {
	s := &Server{}
	if err := s.Configure(config); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (s *Server) Configure(config *ServerConfig) error {
	if config == nil {
		return errors.New("server config cannot be nil")
	}
	ns, err := getNetworkSet(config.InterfaceName)

	if err != nil {
		return err
	}

	s.ServerPort = config.ServerBBMDPort
	s.BroadcastPort = config.ListenPort
	s.DefaultTimeout = config.DefaultTimeout
	s.cHandlersMtx = new(sync.RWMutex)
	s.cHandlers = make(map[string]chan<- *Request)
	s.ucHandlers = make(map[types.UnconfirmedService]map[uint64]chan<- *Request)
	s.ucHandlersMtx = new(sync.RWMutex)
	s.covHandlers = make(map[string]chan<- *Request)
	s.covHandlersMtx = new(sync.RWMutex)
	s.networkSet = ns
	s.close = make(chan struct{})
	s.start = make(chan struct{})
	s.startOnce = sync.Once{}
	s.closeOnce = sync.Once{}
	s.closing = false
	s.listening = false
	s.rcvErrors = false
	s.error = nil

	// Check if user wants to receive errors
	if config.ReceiveErrors {
		// User indicated they want to receive errors
		// Let's create a buffered channel with the
		// size of the total listeners we could have
		s.rcvErrors = true
		s.error = make(chan error, 50)
	}

	return nil
}

func (s *Server) Start() <-chan struct{} {
	return s.start
}

func (s *Server) Close() <-chan struct{} {
	return s.close
}

func (s *Server) Shutdown() {
	s.closeConn()
}

// Errors returns asynchronous listener and socket errors when ReceiveErrors
// is enabled. It returns nil when error reporting is disabled.
func (s *Server) Errors() <-chan error {
	return s.error
}

// Error is retained for compatibility with the original configuration API.
func (s *Server) Error() <-chan error {
	return s.Errors()
}

func (s *Server) GetBroadcastAddr() *net.UDPAddr {
	s.stateMtx.RLock()
	defer s.stateMtx.RUnlock()
	return s.BroadcastAddr
}

func (s *Server) GetBroadcastPort() uint16 {
	return s.BroadcastPort
}

func (s *Server) receiveBroadcast(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	if t, ok := ctx.Deadline(); ok {
		if err := s.BroadcastConn.SetReadDeadline(t); err != nil {
			s.reportError(err)
		}
	}

	go s.receive(s.BroadcastConn, ctx)
}

func (s *Server) receiveUnicast(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	go s.receive(s.UnicastConn, ctx)
}

func (s *Server) receive(conn *net.UDPConn, ctx context.Context) {
	// Loop forever unless we're shutting down
	for {
		select {
		case <-ctx.Done():
			return

		default:
			// Create a byte slice with MAX_MPDU as the length/cap
			b := rxBuffPool.Get().([]byte)

			n, addr, err := conn.ReadFromUDP(b)
			if err != nil {
				rxBuffPool.Put(b)
				if errVal, ok := err.(net.Error); ok {
					if errVal.Timeout() {
						// Timeout error
						continue
					}
				}
				// Send error to an error callback, if we have one registered
				if s.isClosing() || ctx.Err() != nil {
					return
				}
				s.reportError(err)
				return
			} else {
				s.handle(b, n, addr)
				continue
			}
		}
	}
}

// Send covData to a UDP addr
func (s *Server) Send(bytes []byte, dest *net.UDPAddr) error {
	s.stateMtx.RLock()
	conn := s.UnicastConn
	s.stateMtx.RUnlock()
	if conn == nil {
		return errors.New("server is not listening")
	}
	if dest == nil {
		return errors.New("destination cannot be nil")
	}
	_, err := conn.WriteToUDP(bytes, dest)
	return err
}

// Handle a response
func (s *Server) handle(data []byte, n int, address *net.UDPAddr) {
	defer rxBuffPool.Put(data)

	// Ignore any request that originated from our IP address
	// This is most likely going to be a broadcast that we sent.
	if s.isLocalSender(address) {
		// Ignore our broadcasts
		return
	}

	if req, err := ParseRequest(data[:n], address); err != nil {
		// It failed because either we don't know how to decode it
		// or it's an invalid request (spam, random packet ...etc).
		//log.Printf("error decoding response: %s\n", err)
		return
	} else if req.InvokeID() > 0 {
		if req.ServiceChoice() == ConfirmedServiceCovNotification && req.PduType() != PduTypeSimpleAck {
			// This is a cov notification
			if n, ok := req.Apdu.ResponseData.(*pdu.CovNotification); ok {
				if found, delivered := s.deliverCovHandler(address.IP, n.ProcessIdentifier32, req); found {
					if !delivered {
						req.Release()
					}
					return
				}
			} else {
				// oh well..
				// let's just let this request continue and see what happens
			}
		}

		found, delivered := s.deliverConfirmedHandler(address.IP, req.InvokeID(), req)

		if found {
			ReleaseInvokeID(address.IP, req.InvokeID())
			if !delivered {
				req.Release()
			}
		} else {
			req.Release()
		}
	} else {
		if !s.dispatchUnconfirmed(types.UnconfirmedService(req.ServiceChoice()), req, data[:n], address) {
			req.Release()
		}
	}
}

func (s *Server) isLocalSender(address *net.UDPAddr) bool {
	s.stateMtx.RLock()
	serverAddr := s.ServerAddr
	conn := s.UnicastConn
	s.stateMtx.RUnlock()
	if address == nil || serverAddr == nil || !address.IP.Equal(serverAddr.IP) {
		return false
	}
	if conn == nil {
		return address.Port == serverAddr.Port
	}
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	return ok && address.Port == local.Port
}

func deliverRequest(handler chan<- *Request, req *Request) bool {
	select {
	case handler <- req:
		return true
	default:
		return false
	}
}

func (s *Server) getConfirmedHandler(deviceIP net.IP, invokeId uint8) chan<- *Request {
	if invokeId == 0 || deviceIP == nil {
		return nil
	}

	s.cHandlersMtx.RLock()
	defer s.cHandlersMtx.RUnlock()

	if h := s.cHandlers[deviceIP.String()+"."+strconv.Itoa(int(invokeId))]; h != nil {
		return h
	}
	return nil
}

func (s *Server) SetConfirmedHandler(deviceIP net.IP, invokeId uint8, handler chan<- *Request) {
	if invokeId == 0 || deviceIP == nil {
		return
	}

	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()

	s.cHandlers[deviceIP.String()+"."+strconv.Itoa(int(invokeId))] = handler
}

func (s *Server) RemoveConfirmedHandler(deviceIP net.IP, invokeId uint8) {
	if invokeId == 0 || deviceIP == nil {
		return
	}

	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()

	delete(s.cHandlers, deviceIP.String()+"."+strconv.Itoa(int(invokeId)))
}

func (s *Server) deliverConfirmedHandler(deviceIP net.IP, invokeID uint8, req *Request) (bool, bool) {
	if invokeID == 0 || deviceIP == nil {
		return false, false
	}
	key := deviceIP.String() + "." + strconv.Itoa(int(invokeID))
	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()
	h := s.cHandlers[key]
	delete(s.cHandlers, key)
	if h == nil {
		return false, false
	}
	return true, deliverRequest(h, req)
}

func (s *Server) deliverCovHandler(deviceIP net.IP, processID uint32, req *Request) (bool, bool) {
	if deviceIP == nil {
		return false, false
	}
	s.covHandlersMtx.RLock()
	defer s.covHandlersMtx.RUnlock()
	h := s.covHandlers[covHandlerKey(deviceIP, processID)]
	if h == nil {
		return false, false
	}
	return true, deliverRequest(h, req)
}

func (s *Server) getCovHandler(deviceIP net.IP, processId uint32) chan<- *Request {
	if deviceIP == nil {
		return nil
	}

	s.covHandlersMtx.RLock()
	defer s.covHandlersMtx.RUnlock()

	if h := s.covHandlers[covHandlerKey(deviceIP, processId)]; h != nil {
		return h
	}

	return nil
}

func (s *Server) SetCovHandler(deviceIP net.IP, processId uint8, handler chan<- *Request) {
	s.SetCovHandlerWithProcessID(deviceIP, uint32(processId), handler)
}

// SetCovHandlerWithProcessID registers a COV handler using the complete
// BACnet Unsigned32 subscriber process identifier.
func (s *Server) SetCovHandlerWithProcessID(deviceIP net.IP, processId uint32, handler chan<- *Request) {
	if deviceIP == nil {
		return
	}

	s.covHandlersMtx.Lock()
	defer s.covHandlersMtx.Unlock()

	s.covHandlers[covHandlerKey(deviceIP, processId)] = handler
}

func (s *Server) RemoveCovHandler(deviceIP net.IP, processId uint8) {
	s.RemoveCovHandlerWithProcessID(deviceIP, uint32(processId))
}

// RemoveCovHandlerWithProcessID removes a COV handler registered with a full
// BACnet Unsigned32 subscriber process identifier.
func (s *Server) RemoveCovHandlerWithProcessID(deviceIP net.IP, processId uint32) {
	if deviceIP == nil {
		return
	}

	s.covHandlersMtx.Lock()
	defer s.covHandlersMtx.Unlock()

	delete(s.covHandlers, covHandlerKey(deviceIP, processId))
}

func covHandlerKey(deviceIP net.IP, processID uint32) string {
	return deviceIP.String() + "." + strconv.FormatUint(uint64(processID), 10)
}

func (s *Server) getUnconfirmedHandlers(service types.UnconfirmedService) []chan<- *Request {
	s.ucHandlersMtx.RLock()
	defer s.ucHandlersMtx.RUnlock()
	handlers := s.ucHandlers[service]
	result := make([]chan<- *Request, 0, len(handlers))
	for _, h := range handlers {
		result = append(result, h)
	}
	return result
}

func (s *Server) dispatchUnconfirmed(service types.UnconfirmedService, req *Request, data []byte, address *net.UDPAddr) bool {
	s.ucHandlersMtx.RLock()
	defer s.ucHandlersMtx.RUnlock()
	handlers := s.ucHandlers[service]
	if len(handlers) == 0 {
		return false
	}
	first := true
	for _, handler := range handlers {
		message := req
		if !first {
			var err error
			message, err = ParseRequest(data, address)
			if err != nil {
				continue
			}
		}
		first = false
		if !deliverRequest(handler, message) {
			message.Release()
		}
	}
	return true
}

func (s *Server) SetUnconfirmedHandler(service types.UnconfirmedService, handler chan<- *Request) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	s.ucHandlers[service] = map[uint64]chan<- *Request{0: handler}
}

func (s *Server) addUnconfirmedHandler(service types.UnconfirmedService, handler chan<- *Request) uint64 {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	s.ucHandlerID++
	if s.ucHandlerID == 0 {
		s.ucHandlerID++
	}
	if s.ucHandlers[service] == nil {
		s.ucHandlers[service] = make(map[uint64]chan<- *Request)
	}
	s.ucHandlers[service][s.ucHandlerID] = handler
	return s.ucHandlerID
}

func (s *Server) removeUnconfirmedHandler(service types.UnconfirmedService, id uint64) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	delete(s.ucHandlers[service], id)
	if len(s.ucHandlers[service]) == 0 {
		delete(s.ucHandlers, service)
	}
}

func (s *Server) RemoveUnconfirmedHandler(service types.UnconfirmedService) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	if _, exists := s.ucHandlers[service]; exists {
		delete(s.ucHandlers, service)
	}
}

func (s *Server) reportError(err error) {
	if err == nil || !s.rcvErrors || s.error == nil {
		return
	}
	select {
	case s.error <- err:
	default:
	}
}

func (s *Server) isClosing() bool {
	s.stateMtx.RLock()
	defer s.stateMtx.RUnlock()
	return s.closing
}

func (s *Server) markStarted() {
	s.startOnce.Do(func() { close(s.start) })
}

func (s *Server) installConnections(broadcast, unicast *net.UDPConn, serverAddr, broadcastAddr *net.UDPAddr) error {
	s.stateMtx.Lock()
	defer s.stateMtx.Unlock()
	if s.closing {
		return ErrServerShutdown
	}
	s.BroadcastConn = broadcast
	s.UnicastConn = unicast
	s.ServerAddr = serverAddr
	s.BroadcastAddr = broadcastAddr
	return nil
}

func (s *Server) reserveListener() error {
	s.stateMtx.Lock()
	defer s.stateMtx.Unlock()
	if s.closing {
		return ErrServerShutdown
	}
	if s.listening {
		return ErrServerAlreadyListening
	}
	s.listening = true
	return nil
}

func (s *Server) cancelListenerStart() {
	s.stateMtx.Lock()
	s.listening = false
	s.stateMtx.Unlock()
}

func (s *Server) connection() *net.UDPConn {
	s.stateMtx.RLock()
	defer s.stateMtx.RUnlock()
	return s.BroadcastConn
}
