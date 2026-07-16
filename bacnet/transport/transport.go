package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

var (
	ErrClosed    = errors.New("transport is closed")
	ErrQueueFull = errors.New("transport receive queue is full")
)

type Endpoint struct {
	IP      net.IP
	Port    uint16
	Network uint16
	MAC     []byte
}

func NewEndpoint(ip net.IP, port uint16) Endpoint {
	return Endpoint{IP: append(net.IP(nil), ip...), Port: port}
}

func (e Endpoint) String() string {
	if e.Network != 0 {
		return fmt.Sprintf("network=%d mac=%x", e.Network, e.MAC)
	}
	return net.JoinHostPort(e.IP.String(), fmt.Sprint(e.Port))
}

func (e Endpoint) UDPAddr() *net.UDPAddr {
	return &net.UDPAddr{IP: append(net.IP(nil), e.IP...), Port: int(e.Port)}
}

func EndpointFromUDP(address *net.UDPAddr) Endpoint {
	if address == nil {
		return Endpoint{}
	}
	return NewEndpoint(address.IP, uint16(address.Port))
}

type Datagram struct {
	Payload     []byte
	Source      Endpoint
	Destination Endpoint
}

type Conn interface {
	Read(context.Context) (Datagram, error)
	Write(context.Context, Endpoint, []byte) error
	LocalEndpoint() Endpoint
	Close() error
}

type MemoryNetwork struct {
	mu        sync.RWMutex
	listeners map[string]*MemoryConn
}

func NewMemoryNetwork() *MemoryNetwork {
	return &MemoryNetwork{listeners: make(map[string]*MemoryConn)}
}

func (n *MemoryNetwork) Listen(endpoint Endpoint) (*MemoryConn, error) {
	key := endpoint.String()
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.listeners[key]; exists {
		return nil, fmt.Errorf("endpoint %s is already in use", endpoint)
	}
	conn := &MemoryConn{
		network:  n,
		endpoint: cloneEndpoint(endpoint),
		inbound:  make(chan Datagram, 256),
		closed:   make(chan struct{}),
	}
	n.listeners[key] = conn
	return conn, nil
}

func (n *MemoryNetwork) deliver(source, destination Endpoint, payload []byte) error {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if isBroadcast(destination.IP) {
		delivered := false
		for _, listener := range n.listeners {
			if listener.endpoint.Port != destination.Port || listener.endpoint.String() == source.String() {
				continue
			}
			if listener.enqueue(Datagram{Payload: append([]byte(nil), payload...), Source: cloneEndpoint(source), Destination: cloneEndpoint(listener.endpoint)}) {
				delivered = true
			}
		}
		if !delivered {
			return fmt.Errorf("no listeners for broadcast port %d", destination.Port)
		}
		return nil
	}

	listener := n.listeners[destination.String()]
	if listener == nil {
		return fmt.Errorf("endpoint %s is not listening", destination)
	}
	if !listener.enqueue(Datagram{Payload: append([]byte(nil), payload...), Source: cloneEndpoint(source), Destination: cloneEndpoint(destination)}) {
		select {
		case <-listener.closed:
			return ErrClosed
		default:
			return ErrQueueFull
		}
	}
	return nil
}

func (n *MemoryNetwork) remove(conn *MemoryConn) {
	n.mu.Lock()
	delete(n.listeners, conn.endpoint.String())
	n.mu.Unlock()
}

type MemoryConn struct {
	network  *MemoryNetwork
	endpoint Endpoint
	inbound  chan Datagram
	closed   chan struct{}
	close    sync.Once
}

func (c *MemoryConn) Read(ctx context.Context) (Datagram, error) {
	select {
	case <-ctx.Done():
		return Datagram{}, ctx.Err()
	case <-c.closed:
		return Datagram{}, ErrClosed
	case datagram := <-c.inbound:
		return datagram, nil
	}
}

func (c *MemoryConn) Write(ctx context.Context, destination Endpoint, payload []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return ErrClosed
	default:
		return c.network.deliver(c.endpoint, destination, payload)
	}
}

func (c *MemoryConn) LocalEndpoint() Endpoint { return cloneEndpoint(c.endpoint) }

func (c *MemoryConn) Close() error {
	c.close.Do(func() {
		close(c.closed)
		c.network.remove(c)
	})
	return nil
}

func (c *MemoryConn) enqueue(datagram Datagram) bool {
	select {
	case <-c.closed:
		return false
	case c.inbound <- datagram:
		return true
	default:
		return false
	}
}

func cloneEndpoint(endpoint Endpoint) Endpoint {
	endpoint.IP = append(net.IP(nil), endpoint.IP...)
	endpoint.MAC = append([]byte(nil), endpoint.MAC...)
	return endpoint
}

func isBroadcast(ip net.IP) bool {
	if ip == nil {
		return false
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	return v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255
}
