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

	broadcastConn, err := net.ListenUDP("udp4", broadcastAddr)
	if err != nil {
		return err
	}
	unicastConn, err := net.ListenUDP("udp4", serverAddr)
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
