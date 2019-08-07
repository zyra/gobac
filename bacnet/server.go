package bacnet

import (
	"context"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/types"
	"log"
	"net"
	"sync"
	"time"
)

// The main Server object.
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

	close   chan int
	closing bool

	wg *sync.WaitGroup

	rcvErrors bool
	error     chan error

	start chan int

	cHandlersMtx   *sync.RWMutex
	cHandlers      *[255]chan<- *Request
	ucHandlers     map[types.UnconfirmedService]chan<- *Request
	ucHandlersMtx  *sync.RWMutex
	covHandlers    *[255]chan<- *Request
	covHandlersMtx *sync.RWMutex

	concurrency uint
}

// Create a new Server object with the provided configuration
func NewServer(config *ServerConfig) (*Server, error) {
	ns, err := getNetworkSet(config.InterfaceName)

	if err != nil {
		return nil, err
	}

	s := &Server{
		ServerPort:     config.ServerBBMDPort,
		BroadcastPort:  config.ListenPort,
		DefaultTimeout: config.DefaultTimeout,
		cHandlersMtx:   new(sync.RWMutex),
		cHandlers:      &[255]chan<- *Request{},
		ucHandlers:     make(map[types.UnconfirmedService]chan<- *Request),
		ucHandlersMtx:  new(sync.RWMutex),
		covHandlers:    &[255]chan<- *Request{},
		covHandlersMtx: new(sync.RWMutex),
		networkSet:     ns,
		concurrency:    config.Concurrency,
		close:          make(chan int),
		start:          make(chan int),
		wg:             &sync.WaitGroup{},
	}

	// Check if user wants to receive errors
	if config.ReceiveErrors {
		// User indicated they want to receive errors
		// Let's create a buffered channel with the
		// size of the total listeners we could have
		s.rcvErrors = true
		s.error = make(chan error, config.Concurrency*2)
	}

	return s, nil
}

// Start listening with the provided context.
// This method will block until the context is marked as done.
//
// If you want to start listening and then do some other processing,
// you can start the Server in a goroutine and then listen to the
// .Start() channel that will fire as soon as the Server is up
// and the listeners are up.
//
// This method will panic if any of the listeners fail to start.
// This would typically happen if the address or port you are
// trying to bind to is already taken.
func (s *Server) Listen(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.ServerAddr = getUdpAddr(s.IPv4, s.ServerPort)
	s.BroadcastAddr = getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	if conn, err := net.ListenUDP("udp", s.BroadcastAddr); err != nil {
		panic(err)
	} else {
		s.BroadcastConn = conn
		if err := conn.SetReadBuffer(types.MaxApdu); err != nil && s.rcvErrors {
			s.error <- err
		}
	}

	if conn, err := net.ListenUDP("udp", s.ServerAddr); err != nil {
		panic(err)
	} else {
		s.UnicastConn = conn
		if err := conn.SetReadBuffer(types.MaxMpdu); err != nil && s.rcvErrors {
			s.error <- err
		}
	}

	if deadline, ok := ctx.Deadline(); ok {
		// Context has a deadline. Let's set the deadline of our connection read
		// this this value.
		if err := s.UnicastConn.SetReadDeadline(deadline); err != nil {
			if s.rcvErrors {
				s.error <- err
			}
		}

		if err := s.UnicastConn.SetWriteDeadline(deadline); err != nil {
			if s.rcvErrors {
				s.error <- err
			}
		}
	}

	s.ReceiveBroadcast(ctx)
	s.ReceiveUnicast(ctx)

	s.start <- 1
	close(s.start)

	<-ctx.Done()
	s.closeConn()
}

func (s *Server) Start() <-chan int {
	return s.start
}

func (s *Server) Close() <-chan int {
	return s.close
}

func (s *Server) closeConn() {
	s.closing = true
	if err := s.UnicastConn.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	if err := s.BroadcastConn.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	s.close <- 1
	s.start = make(chan int)
}

func (s *Server) ReceiveBroadcast(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	if t, ok := ctx.Deadline(); ok {
		if err := s.BroadcastConn.SetReadDeadline(t); err != nil {
			if s.rcvErrors {
				s.error <- err
			}
		}
	}

	for i := uint(0); i < s.concurrency; i++ {
		go s.receive(s.BroadcastConn, ctx)
	}
}

func (s *Server) ReceiveUnicast(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	for i := uint(0); i < s.concurrency; i++ {
		go s.receive(s.UnicastConn, ctx)
	}
}

func (s *Server) receive(conn *net.UDPConn, ctx context.Context) {
	// Create a byte slice with MAX_MPDU as the length/cap
	// We will re-use this for every request
	b := make([]byte, types.MaxMpdu, types.MaxMpdu)

	// Loop forever unless we're shutting down
	for {
		select {
		case <-ctx.Done():
			b = nil
			return

		default:
			if n, addr, err := conn.ReadFromUDP(b); err != nil {
				if errVal, ok := err.(net.Error); ok {
					if errVal.Timeout() {
						// Timeout error
						continue
					}
				}
				// Send error to an error callback, if we have one registered
				if !s.closing && s.rcvErrors {
					s.error <- err
				}
			} else {
				// Send a copy of the bytes to the handle function along with
				// the IP address of the sender.
				//
				// Let's also splice our byte slice to only Send the bytes
				// that were written by this request.
				go s.handle(b[:n], addr)
			}
		}
	}
}

// Send covData to a UDP addr
func (s *Server) Send(bytes []byte, dest *net.UDPAddr) error {
	_, err := s.UnicastConn.WriteToUDP(bytes, dest)
	return err
}

// Handle a response
func (s *Server) handle(data []byte, address *net.UDPAddr) {
	// Ignore any request that originated from our IP address
	// This is most likely going to be a broadcast that we sent.
	if address.IP.Equal(s.ServerAddr.IP) {
		// Ignore our broadcasts
		return
	}

	if req, err := ParseRequest(data, &address.IP); err != nil {
		// It failed because either we don't know how to decode it
		// or it's an invalid request (spam, random packet ...etc).
		log.Printf("error decoding response: %s\n", err)
		return
	} else if req.InvokeID() > 0 {
		ReleaseInvokeID(req.InvokeID())

		if req.ServiceChoice() == ConfirmedServiceCovNotification && req.PduType() != PduTypeSimpleAck {
			// This is a cov notification
			if n, ok := req.Apdu.ResponseData.(*pdu.CovNotification); ok {
				if h := s.getCovHandler(n.ProcessIdentifier); h != nil {
					h <- req
					return
				} else {
					// Probably an old subscription that's not valid anymore
					// let's unsubscribe and stop this madness
					println("cancelling a CoV", req.InvokeID(), n.ProcessIdentifier)
					_, _ = s.SubscribeCov(&address.IP, n.ObjectId.Type, n.ObjectId.Instance, n.ProcessIdentifier, true)
					return
				}
			} else {
				// oh well..
				// let's just let this request continue and see what happens
			}
		}

		h := s.getConfirmedHandler(req.InvokeID())

		if h != nil {
			h <- req
			//close(h)
			//s.RemoveConfirmedHandler(req.InvokeID())
		} else {
			//log.Printf("no handler was registered for invoke id %d, ignoring this message\n", req.InvokeID())
		}
	} else {
		h := s.getUnconfirmedHandler(req.ServiceChoice())
		if h != nil {
			h <- req
			//close(h)
			//s.RemoveUnconfirmedHandler(req.ServiceChoice())
		} else {
			//log.Printf("no handler was registered for unconfirmed choice %d, ignoring this message\n", req.ServiceChoice())
		}
	}
}

func (s *Server) getConfirmedHandler(invokeId uint8) chan<- *Request {
	if invokeId == 0 {
		//fmt.Println("getConfirmedHandler got invokeId 0!")
		return nil
	}

	if h := s.cHandlers[invokeId-1]; h != nil {
		return h
	}
	return nil
}

func (s *Server) SetConfirmedHandler(invokeId uint8, handler chan<- *Request) {
	if invokeId == 0 {
		//fmt.Println("SetConfirmedHandler got invokeId 0!")
		return
	}

	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()
	s.cHandlers[invokeId-1] = handler
}

func (s *Server) RemoveConfirmedHandler(invokeId uint8) {
	if invokeId == 0 {
		//fmt.Println("RemoveConfirmedHandler got invokeId 0!")
		return
	}

	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()
	s.cHandlers[invokeId-1] = nil
}

func (s *Server) getCovHandler(processId uint8) chan<- *Request {
	if processId == 0 {
		//fmt.Println("getCovHandler got processId 0!")
		return nil
	}
	s.covHandlersMtx.RLock()
	defer s.covHandlersMtx.RUnlock()
	if h := s.covHandlers[processId-1]; h != nil {
		return h
	}
	return nil
}

func (s *Server) SetCovHandler(processId uint8, handler chan<- *Request) {
	if processId == 0 {
		//fmt.Println("SetCovHandler got processId 0!")
		return
	}

	s.covHandlersMtx.Lock()
	defer s.covHandlersMtx.Unlock()
	s.covHandlers[processId-1] = handler
}

func (s *Server) RemoveCovHandler(processId uint8) {
	if processId == 0 {
		//fmt.Println("RemoveCovHandler got processId 0!")
		return
	}

	s.covHandlersMtx.Lock()
	defer s.covHandlersMtx.Unlock()
	s.covHandlers[processId-1] = nil
}

func (s *Server) getUnconfirmedHandler(service types.UnconfirmedService) chan<- *Request {
	s.ucHandlersMtx.RLock()
	defer s.ucHandlersMtx.RUnlock()
	if h, exists := s.ucHandlers[service]; exists {
		return h
	}
	return nil
}

func (s *Server) SetUnconfirmedHandler(service types.UnconfirmedService, handler chan<- *Request) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	s.ucHandlers[service] = handler
}

func (s *Server) RemoveUnconfirmedHandler(service types.UnconfirmedService) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	if _, exists := s.ucHandlers[service]; exists {
		delete(s.ucHandlers, service)
	}
}
