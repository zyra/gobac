//go:build !linux && !freebsd
// +build !linux,!freebsd

package bacnet

import (
	"context"
	"net"
)

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

	// The broadcast socket binds the wildcard address, not the literal
	// directed-broadcast address (broadcastAddr, e.g. 127.255.255.255):
	// unlike Linux, BSD-family kernels (including darwin) refuse to bind a
	// UDP socket to a directed-broadcast IP at all ("bind: can't assign
	// requested address") since it isn't a real local interface address.
	// A wildcard bind still receives datagrams sent to that broadcast
	// address, matching how server_posix.go's single Linux/FreeBSD socket
	// binds ":"+port. broadcastAddr itself is unaffected: it is still
	// recorded via installConnections below as the destination address
	// used when sending broadcasts.
	broadcastBindAddr := getUdpAddr(net.IPv4zero, s.BroadcastPort)

	broadcastConn, err := listenBroadcastUDP(ctx, broadcastBindAddr)
	if err != nil {
		return err
	}
	unicastConn, err := listenBroadcastUDP(ctx, serverAddr)
	if err != nil {
		_ = broadcastConn.Close()
		return err
	}
	if err := s.installConnections(broadcastConn, unicastConn, serverAddr, broadcastAddr); err != nil {
		_ = unicastConn.Close()
		_ = broadcastConn.Close()
		return err
	}
	installed = true

	s.receiveBroadcast(ctx)
	s.receiveUnicast(ctx)
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
		unicastConn := s.UnicastConn
		broadcastConn := s.BroadcastConn
		s.stateMtx.Unlock()
		if unicastConn != nil {
			if err := unicastConn.Close(); err != nil {
				s.reportError(err)
			}
		}
		if broadcastConn != nil {
			if err := broadcastConn.Close(); err != nil {
				s.reportError(err)
			}
		}
		close(s.close)
	})
}

func (s *Server) GetConnection() *net.UDPConn {
	return s.connection()
}
