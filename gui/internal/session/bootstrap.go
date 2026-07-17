package session

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
func StartFromSettings(s Starter, iface string, port int) error {
	return s.Start(ConfigFromSettings(iface, port))
}

// Shutdown stops s as part of an application close path (e.g. a window
// close intercept). It is a named seam rather than an inline s.Stop() call
// so that shutdown wiring can be unit-tested against a fake Starter.
func Shutdown(s Starter) error {
	return s.Stop()
}
