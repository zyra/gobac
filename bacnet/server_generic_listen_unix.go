//go:build aix || darwin || dragonfly || netbsd || openbsd || solaris
// +build aix darwin dragonfly netbsd openbsd solaris

package bacnet

import (
	"context"
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// listenBroadcastUDP creates a UDP socket with SO_REUSEADDR, SO_REUSEPORT
// (where supported — see setReusePort) and SO_BROADCAST set, matching the
// linux/freebsd listener in server_posix.go and the transport package
// listeners.
//
// Unlike server_posix.go, ListenContext on this platform set (see
// server_generic.go) binds two sockets that share one port: a unicast
// socket on the interface's own address and a broadcast socket on the
// interface's directed-broadcast address (e.g. 127.0.0.1:p and
// 127.255.255.255:p for a loopback interface). SO_REUSEADDR alone is
// sufficient on Linux/FreeBSD, which never reach this file (they use a
// single wildcard socket instead), but the BSD-family kernels that do use
// this path require SO_REUSEPORT as well to allow that same-port,
// different-address pair to bind without EADDRINUSE/EADDRNOTAVAIL.
func listenBroadcastUDP(ctx context.Context, addr *net.UDPAddr) (*net.UDPConn, error) {
	listenConfig := net.ListenConfig{
		Control: func(network, address string, raw syscall.RawConn) error {
			var controlErr error
			if err := raw.Control(func(fd uintptr) {
				controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				if controlErr == nil {
					controlErr = setReusePort(int(fd))
				}
				if controlErr == nil {
					controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
				}
			}); err != nil {
				return err
			}
			return controlErr
		},
	}
	packetConn, err := listenConfig.ListenPacket(ctx, "udp4", addr.String())
	if err != nil {
		return nil, err
	}
	conn, ok := packetConn.(*net.UDPConn)
	if !ok {
		_ = packetConn.Close()
		return nil, errors.New("listener did not create a UDP connection")
	}
	return conn, nil
}
