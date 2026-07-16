package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

type UDPConn struct {
	conn     *net.UDPConn
	endpoint Endpoint
	close    sync.Once
}

func ListenUDP(endpoint Endpoint) (*UDPConn, error) {
	conn, err := listenUDP(endpoint.UDPAddr())
	if err != nil {
		return nil, err
	}
	local := EndpointFromUDP(conn.LocalAddr().(*net.UDPAddr))
	return &UDPConn{conn: conn, endpoint: local}, nil
}

func (c *UDPConn) Read(ctx context.Context) (Datagram, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			return Datagram{}, err
		}
	} else {
		if err := c.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return Datagram{}, err
		}
	}

	buffer := make([]byte, 65535)
	for {
		n, source, err := c.conn.ReadFromUDP(buffer)
		if err == nil {
			return Datagram{
				Payload:     append([]byte(nil), buffer[:n]...),
				Source:      EndpointFromUDP(source),
				Destination: c.LocalEndpoint(),
			}, nil
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			select {
			case <-ctx.Done():
				return Datagram{}, ctx.Err()
			default:
				if _, ok := ctx.Deadline(); ok {
					return Datagram{}, err
				}
				if err := c.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
					return Datagram{}, err
				}
				continue
			}
		}
		return Datagram{}, err
	}
}

func (c *UDPConn) Write(ctx context.Context, destination Endpoint, payload []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetWriteDeadline(deadline); err != nil {
			return err
		}
	} else {
		if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	_, err := c.conn.WriteToUDP(payload, destination.UDPAddr())
	return err
}

func (c *UDPConn) LocalEndpoint() Endpoint { return cloneEndpoint(c.endpoint) }

func (c *UDPConn) Close() error {
	var err error
	c.close.Do(func() { err = c.conn.Close() })
	return err
}
