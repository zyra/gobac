//go:build windows
// +build windows

package bacnet

import (
	"context"
	"errors"
	"net"
	"syscall"
)

// listenBroadcastUDP creates a UDP socket with SO_REUSEADDR and SO_BROADCAST
// set, matching the linux/freebsd listener in server_posix.go and the
// transport package listeners.
func listenBroadcastUDP(ctx context.Context, addr *net.UDPAddr) (*net.UDPConn, error) {
	listenConfig := net.ListenConfig{
		Control: func(network, address string, raw syscall.RawConn) error {
			var controlErr error
			if err := raw.Control(func(fd uintptr) {
				controlErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if controlErr == nil {
					controlErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
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
