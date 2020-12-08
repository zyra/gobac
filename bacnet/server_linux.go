// +build linux freebsd

package bacnet

import (
	"context"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"syscall"
)

func (s *server) Listen(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.ServerAddr = getUdpAddr(s.IPv4, s.ServerPort)
	s.BroadcastAddr = getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)

	if err != nil {
		log.Fatal(err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		log.Fatal(err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_BROADCAST, 1); err != nil {
		log.Fatal(err)
	}

	addr := syscall.SockaddrInet4{
		Port: int(s.BroadcastPort),
		//Addr: [4]byte{
		//	(*s.IPv4)[0],
		//	(*s.IPv4)[1],
		//	(*s.IPv4)[2],
		//	(*s.IPv4)[3],
		//},
	}

	if err := syscall.Bind(fd, &addr); err != nil {
		_ = syscall.Close(fd)
		log.Fatal(err)
	}

	socketFile := os.NewFile(uintptr(fd), "")

	l, e := net.FilePacketConn(socketFile)

	_ = socketFile.Close()

	if e != nil {
		log.Fatal(e)
	}

	if udpConn, ok := l.(*net.UDPConn); !ok {
		log.Fatal("can't cast conn to udp conn")
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

func (s *server) GetConnection() *net.UDPConn {
	return s.BroadcastConn
}
