// +build linux freebsd

package bacnet

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"syscall"
)

func (s *server) Listen(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.ServerAddr = getUdpAddr(s.IPv4, s.ServerPort)
	s.BroadcastAddr = getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	listenConfig := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
				panic(err)
			}

			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
				panic(err)
			}
		})
	}}

	l, e := listenConfig.ListenPacket(ctx, "udp", fmt.Sprintf(":%d", s.BroadcastPort))

	if e != nil {
		panic(e)
	}

	if udpConn, ok := l.(*net.UDPConn); !ok {
		panic("can't cast conn to udp conn")
	} else {
		s.BroadcastConn = udpConn
		s.UnicastConn = udpConn

		s.receiveBroadcast(ctx)
		s.receiveUnicast(ctx)

		close(s.start)

		<-ctx.Done()
		s.closeConn()
	}
}

func (s *server) closeConn() {
	s.closing = true
	if err := s.BroadcastConn.Close(); err != nil {
		log.Printf("Error closing connection: %s\n", err)
	}
	s.close <- struct{}{}
	s.start = make(chan struct{})
}
