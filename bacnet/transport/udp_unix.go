//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package transport

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func listenUDP(address *net.UDPAddr) (*net.UDPConn, error) {
	config := net.ListenConfig{Control: func(network, address string, raw syscall.RawConn) error {
		var socketErr error
		if err := raw.Control(func(fd uintptr) {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
				socketErr = err
				return
			}
			socketErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
		}); err != nil {
			return err
		}
		return socketErr
	}}
	packet, err := config.ListenPacket(context.Background(), "udp4", address.String())
	if err != nil {
		return nil, err
	}
	conn, ok := packet.(*net.UDPConn)
	if !ok {
		packet.Close()
		return nil, &net.AddrError{Err: "unexpected packet connection type", Addr: address.String()}
	}
	return conn, nil
}
