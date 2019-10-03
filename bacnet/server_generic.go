// +build !linux
// +build !freebsd

package bacnet

import (
	"context"
	"log"
	"net"
)

func (s *server) Listen(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.ServerAddr = getUdpAddr(s.IPv4, s.ServerPort)
	s.BroadcastAddr = getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	if conn, err := net.ListenUDP("udp", s.BroadcastAddr); err != nil {
		panic(err)
	} else {
		s.BroadcastConn = conn
	}

	if conn, err := net.ListenUDP("udp", s.ServerAddr); err != nil {
		panic(err)
	} else {
		s.UnicastConn = conn
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

	s.receiveBroadcast(ctx)
	s.receiveUnicast(ctx)

	close(s.start)

	<-ctx.Done()
	s.closeConn()
}

func (s *server) closeConn() {
	s.closing = true
	if err := s.UnicastConn.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	if err := s.BroadcastConn.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	s.close <- struct{}{}
	s.start = make(chan struct{})
}
