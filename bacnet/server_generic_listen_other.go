//go:build !aix && !darwin && !dragonfly && !netbsd && !openbsd && !solaris && !windows && !linux && !freebsd
// +build !aix,!darwin,!dragonfly,!netbsd,!openbsd,!solaris,!windows,!linux,!freebsd

package bacnet

import (
	"context"
	"net"
)

// listenBroadcastUDP falls back to a plain UDP listener on platforms where
// SO_BROADCAST plumbing has not been implemented (e.g. plan9, js/wasm).
func listenBroadcastUDP(_ context.Context, addr *net.UDPAddr) (*net.UDPConn, error) {
	return net.ListenUDP("udp4", addr)
}
