//go:build solaris
// +build solaris

package bacnet

// setReusePort is a no-op on solaris: unix.SO_REUSEPORT is not defined
// there in the vendored golang.org/x/sys/unix, and solaris is not part of
// this repository's build/test matrix (see server_generic_listen_unix.go).
func setReusePort(fd int) error {
	return nil
}
