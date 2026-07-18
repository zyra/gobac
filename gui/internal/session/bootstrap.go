package session

import (
	"errors"
	"net"

	"github.com/zyra/gobac/gui/internal/netpick"
)

// resolveAutomatic picks the interface to use when the user has left the
// network setting on "Automatic": the best candidate netpick finds among
// the real OS interfaces. It's a func-var seam so bootstrap tests can
// substitute a fake candidate set without touching real network interfaces.
var resolveAutomatic = func() (netpick.Candidate, bool) {
	return netpick.Automatic(netpick.Candidates(net.Interfaces))
}

// AllNetworksSentinel is the persisted Settings.Interface value meaning
// "listen on all networks" (task U5): StartFromSettings maps it straight to
// the bacnet library's wildcard Config.Interface ("") rather than resolving
// it through Automatic. See bacnet/util.go's getNetworkSet("") for what the
// wildcard actually does (binds 0.0.0.0, sends Who-Is to the limited
// broadcast address 255.255.255.255).
const AllNetworksSentinel = "*"

// Starter is the subset of Session that application startup/shutdown needs:
// just enough to start a session from persisted settings and stop it again
// on exit. Live satisfies it; tests substitute a fake so this wiring is
// covered without binding real sockets.
type Starter interface {
	Start(cfg Config) error
	Stop() error
}

// ConfigFromSettings maps a persisted network interface name and UDP port
// into a Config. LocalPort mirrors Port: the session identifies itself on
// the same port it listens on (see Config.LocalPort).
func ConfigFromSettings(iface string, port int) Config {
	p := uint16(port)
	return Config{Interface: iface, Port: p, LocalPort: p}
}

// StartFromSettings maps iface/port into a Config and starts s, returning
// whatever error Start produced unchanged. Callers (the GUI's startup path)
// are expected to surface that error non-fatally, e.g. in a status bar,
// rather than aborting launch: other views such as the simulator quickstart
// and scenario editor don't depend on a running session.
//
// AllNetworksSentinel ("*") maps straight to the bacnet library's wildcard
// Config.Interface (""), bypassing Automatic entirely. An empty iface means
// "Automatic": it resolves to the best real interface via resolveAutomatic
// before starting, failing with a plain "no usable network found" if none
// is available. Any other iface passes through unchanged.
func StartFromSettings(s Starter, iface string, port int) error {
	if iface == AllNetworksSentinel {
		return s.Start(ConfigFromSettings("", port))
	}
	if iface == "" {
		c, ok := resolveAutomatic()
		if !ok {
			return errors.New("no usable network found")
		}
		iface = c.Name
	}
	return s.Start(ConfigFromSettings(iface, port))
}

// Shutdown stops s as part of an application close path (e.g. a window
// close intercept). It is a named seam rather than an inline s.Stop() call
// so that shutdown wiring can be unit-tested against a fake Starter.
func Shutdown(s Starter) error {
	return s.Stop()
}
