package gobac

import (
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"log"
	"net"
	"sync"
	"time"
)

type Operation struct {
	InvokeID uint8
	Timeout  time.Duration
	Tx       chan Response
	Rx       chan int
	Done     chan int
}

type responseHandler = func(response *Response)

type Server struct {
	*networkSet
	ServerAddr    *net.UDPAddr
	ServerPort    uint16
	BroadcastAddr *net.UDPAddr
	BroadcastPort uint16
	UConnection   *net.UDPConn // Unicast
	BConnection   *net.UDPConn // Broadcast
	Timeout       time.Duration
	Operations    *[]*Operation
	Close         chan int
	ErrorCallback func(err error)
	cHandlers     map[types.ConfirmedService]responseHandler
	cHandlersMtx  sync.RWMutex
	ucHandlers    map[types.UnconfirmedService]responseHandler
	ucHandlersMtx sync.RWMutex
}

func NewServer(ifname string) (*Server, error) {
	ns, err := getNetworkSet(ifname)

	if err != nil {
		return nil, err
	}

	ops := make([]*Operation, 0, 255)

	s := &Server{
		ServerPort:    0xBAC0,
		BroadcastPort: 0xBAC0,
		Timeout:       time.Second * 10,
		Operations:    &ops,
		cHandlers:     make(map[types.ConfirmedService]responseHandler),
		ucHandlers:    make(map[types.UnconfirmedService]responseHandler),
		networkSet:    ns,
	}

	return s, nil
}

func (s *Server) Listen() {
	s.ServerAddr = getUdpAddr(s.IPv4, s.ServerPort)
	s.BroadcastAddr = getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	if conn, err := net.ListenUDP("udp", s.BroadcastAddr); err != nil {
		panic(err)
	} else {
		s.BConnection = conn
	}

	if conn, err := net.ListenUDP("udp", s.ServerAddr); err != nil {
		panic(err)
	} else {
		s.UConnection = conn
	}

	go s.receiveUnicast()
	go s.receiveBroadcast()
}

func (s *Server) closeConn() {
	if err := s.UConnection.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	if err := s.BConnection.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
}

func (s *Server) receiveUnicast() {
	b := make([]byte, types.MAX_MPDU)
	if _, addr, err := s.UConnection.ReadFromUDP(b); err != nil {
		if s.ErrorCallback != nil {
			s.ErrorCallback(err)
		}
	} else {
		go s.handle(b, addr)
		s.receiveUnicast()
	}
}

func (s *Server) SendMPDU(mtu *encoding.Buffer, dest *net.UDPAddr) error {
	_, err := s.UConnection.WriteToUDP(mtu.Bytes(), dest)
	return err
}

func (s *Server) receiveBroadcast() {
	b := make([]byte, types.MAX_MPDU)
	if _, addr, err := s.BConnection.ReadFromUDP(b[:]); err != nil {
		if s.ErrorCallback != nil {
			s.ErrorCallback(err)
		}
	} else {
		go s.handle(b, addr)
		s.receiveBroadcast()
	}
}

func (s *Server) handle(data []byte, address *net.UDPAddr) {
	res := NewPduResponse(data)
	res.Sender = address
	if err := res.Decode(); err != nil {
		log.Printf("error decoding response: %s\n", err)
		return
	}

	switch res.PduType {
	case types.PDU_TYPE_UNCONFIRMED_SERVICE_REQUEST:
		h := s.getUnconfirmedHandler(res.Choice)
		if h != nil {
			(*h)(res)
		} else {
			log.Printf("no handler was registered for pdu type %d, ignoring this message\n", res.PduType)
		}
		break

	case types.PDU_TYPE_CONFIRMED_SERVICE_REQUEST:
		h := s.getConfirmedHandler(res.Choice)
		if h != nil {
			(*h)(res)
		} else {
			log.Printf("no handler was registered for pdu type %d, ignoring this message\n", res.PduType)
		}
		break

	default:
		res.Valid = false
		log.Printf("unsupported pdu type: %d; ignoring this message\n", res.PduType)
	}
}

func (s *Server) getConfirmedHandler(service types.ConfirmedService) *responseHandler {
	s.cHandlersMtx.RLock()
	defer s.cHandlersMtx.RUnlock()
	if h, exists := s.cHandlers[service]; exists {
		return &h
	}
	return nil
}

func (s *Server) setConfirmedHandler(service types.ConfirmedService, handler responseHandler) {
	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()
	s.cHandlers[service] = handler
}

func (s *Server) removeConfirmedHandler(service types.ConfirmedService) {
	s.cHandlersMtx.Lock()
	defer s.cHandlersMtx.Unlock()
	delete(s.cHandlers, service)
}

func (s *Server) getUnconfirmedHandler(service types.UnconfirmedService) *responseHandler {
	s.ucHandlersMtx.RLock()
	defer s.ucHandlersMtx.RUnlock()
	if h, exists := s.ucHandlers[service]; exists {
		return &h
	}
	return nil
}

func (s *Server) setUnconfirmedHandler(service types.UnconfirmedService, handler responseHandler) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	s.ucHandlers[service] = handler
}

func (s *Server) removeUnconfirmedHandler(service types.UnconfirmedService) {
	s.ucHandlersMtx.Lock()
	defer s.ucHandlersMtx.Unlock()
	delete(s.ucHandlers, service)
}
