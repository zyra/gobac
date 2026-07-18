//go:build aix || darwin || dragonfly || netbsd || openbsd
// +build aix darwin dragonfly netbsd openbsd

package bacnet

import "golang.org/x/sys/unix"

// setReusePort sets SO_REUSEPORT on fd. It is split out from
// server_generic_listen_unix.go because unix.SO_REUSEPORT is not defined for
// every GOOS that file's build tag covers (notably solaris; see
// server_generic_listen_reuseport_other.go).
func setReusePort(fd int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}
