//go:build linux || freebsd
// +build linux freebsd

package bacnet

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
)

// Listen starts the BACnet/IP listener and reports startup errors through
// Errors when configured. ListenContext is preferred when startup errors need
// to be returned directly.
func (s *Server) Listen(ctx context.Context) {
	if err := s.ListenContext(ctx); err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			s.reportError(err)
		}
		s.markStarted()
		if err != ErrServerAlreadyListening {
			s.closeConn()
		}
	}
}

// ListenContext starts the BACnet/IP listener and blocks until cancellation or
// shutdown. Linux and FreeBSD use one socket because it receives both unicast
// and broadcast datagrams for the bound BACnet port.
func (s *Server) ListenContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.reserveListener(); err != nil {
		return err
	}
	installed := false
	defer func() {
		if !installed {
			s.cancelListenerStart()
		}
	}()

	serverAddr := getUdpAddr(s.IPv4, s.ServerPort)
	broadcastAddr := getUdpAddr(s.BroadcastIPv4, s.BroadcastPort)

	listenConfig := net.ListenConfig{
		Control: func(network, address string, raw syscall.RawConn) error {
			var controlErr error
			if err := raw.Control(func(fd uintptr) {
				controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if controlErr == nil {
					controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
				}
			}); err != nil {
				return err
			}
			return controlErr
		},
	}
	packetConn, err := listenConfig.ListenPacket(ctx, "udp4", ":"+strconv.Itoa(int(s.BroadcastPort)))
	if err != nil {
		return err
	}
	conn, ok := packetConn.(*net.UDPConn)
	if !ok {
		_ = packetConn.Close()
		return errors.New("listener did not create a UDP connection")
	}
	if err := s.installConnections(conn, conn, serverAddr, broadcastAddr); err != nil {
		_ = conn.Close()
		return err
	}
	installed = true

	s.receiveBroadcast(ctx)
	s.markStarted()

	select {
	case <-ctx.Done():
		s.closeConn()
		return ctx.Err()
	case <-s.close:
		return nil
	}
}

func (s *Server) closeConn() {
	s.closeOnce.Do(func() {
		s.stateMtx.Lock()
		s.closing = true
		s.listening = false
		conn := s.BroadcastConn
		s.stateMtx.Unlock()
		if conn != nil {
			if err := conn.Close(); err != nil {
				s.reportError(err)
			}
		}
		close(s.close)
	})
}

func (s *Server) GetConnection() *net.UDPConn {
	return s.connection()
}
