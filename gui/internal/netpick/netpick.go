// Package netpick turns the OS network interface list into a small set of
// human-friendly candidates for the settings UI: which interfaces are usable
// (up, with an IPv4 address), what to call them, and which one to pick
// automatically when the user hasn't chosen one. It has no dependency on
// Fyne so it stays unit-testable on its own, matching the rest of this
// module's non-ui packages.
package netpick

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// Candidate is one selectable network interface.
type Candidate struct {
	// Name is the OS interface name (e.g. "eno0", "lo"). This is what gets
	// persisted to settings -- never the Label, which embeds a volatile IP.
	Name string
	// Label is the human-friendly display string, e.g.
	// "Wired (eno0) — 192.168.1.5".
	Label string
	// IPv4 is the interface's first usable IPv4 address, in dotted form.
	IPv4 string
	// Loopback reports whether this is a loopback interface (used by the
	// simulator, so it's listed rather than filtered out).
	Loopback bool
	// Virtual reports whether Name matches a common virtual/container
	// interface prefix (docker bridges, veth pairs, VPN tunnels, etc). It
	// doesn't affect availability in the dropdown -- these are still real,
	// selectable candidates -- only ranking: Automatic and the sorted
	// candidate list both prefer a physical NIC when one is present, so a
	// docker0 bridge never silently wins over the machine's real LAN
	// interface just because its name sorts first alphabetically.
	Virtual bool
}

// virtualPrefixes are OS interface name prefixes that commonly identify a
// virtual, container, or tunnel interface rather than a physical NIC:
// Docker/container bridges and veth pairs, libvirt's virbr, CNI/Flannel
// pod networking, Tailscale/WireGuard/generic tun-tap VPN interfaces, and
// ZeroTier. Matching one of these only affects ranking (see
// Candidate.Virtual) -- it never excludes the interface from the list.
var virtualPrefixes = []string{
	"docker", "br-", "veth", "virbr", "cni", "flannel",
	"tailscale", "tun", "tap", "wg", "zt",
}

// isVirtual reports whether name matches one of virtualPrefixes.
func isVirtual(name string) bool {
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// Candidates lists the usable network interfaces: those reported up by
// list (production passes net.Interfaces) that have at least one IPv4
// address. list is a seam so tests can supply a fake interface set without
// touching real hardware. Results are sorted non-loopback first, then by
// name.
func Candidates(list func() ([]net.Interface, error)) []Candidate {
	ifaces, err := list()
	if err != nil {
		return nil
	}

	var out []Candidate
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		ipv4, ok := firstIPv4(addrs)
		if !ok {
			continue
		}
		loopback := ifc.Flags&net.FlagLoopback != 0
		out = append(out, Candidate{
			Name:     ifc.Name,
			Label:    label(ifc.Name, ipv4, loopback),
			IPv4:     ipv4,
			Loopback: loopback,
			Virtual:  isVirtual(ifc.Name),
		})
	}

	sortCandidates(out)

	return out
}

// sortCandidates orders cands non-loopback first, then physical (non-
// virtual) interfaces before virtual ones, then by name, in place. This
// keeps every candidate available and visible in the dropdown -- it only
// controls display/pick order, so a docker0 bridge still ranks below the
// real LAN NIC it would otherwise beat alphabetically.
func sortCandidates(cands []Candidate) {
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Loopback != cands[j].Loopback {
			return !cands[i].Loopback
		}
		if cands[i].Virtual != cands[j].Virtual {
			return !cands[i].Virtual
		}
		return cands[i].Name < cands[j].Name
	})
}

// Automatic picks the candidate the GUI should use when the user has not
// chosen one explicitly: the first physical (non-loopback, non-virtual)
// candidate, falling back to the first non-loopback candidate (which may
// be virtual, e.g. a docker-only host), then the first loopback candidate,
// and reporting false when cands is empty. This ranks a real LAN NIC ahead
// of docker/bridge/tunnel interfaces without ever hiding the latter from
// the dropdown -- see Candidate.Virtual.
func Automatic(cands []Candidate) (Candidate, bool) {
	for _, c := range cands {
		if !c.Loopback && !c.Virtual {
			return c, true
		}
	}
	for _, c := range cands {
		if !c.Loopback {
			return c, true
		}
	}
	for _, c := range cands {
		if c.Loopback {
			return c, true
		}
	}
	return Candidate{}, false
}

// firstIPv4 returns the first IPv4 address among addrs, in dotted form.
func firstIPv4(addrs []net.Addr) (string, bool) {
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4.String(), true
		}
	}
	return "", false
}

// friendlyName maps an interface name to a human-friendly kind, by prefix:
// "wl*" (wireless) -> "Wi-Fi", "en*"/"eth*" (wired) -> "Wired", "lo*"
// (loopback) -> "Loopback (testing)", anything else -> "Network".
func friendlyName(name string) string {
	switch {
	case strings.HasPrefix(name, "wl"):
		return "Wi-Fi"
	case strings.HasPrefix(name, "en"), strings.HasPrefix(name, "eth"):
		return "Wired"
	case strings.HasPrefix(name, "lo"):
		return "Loopback (testing)"
	default:
		return "Network"
	}
}

// label formats a candidate's display string: "<Friendly> (<name>) —
// <IPv4>" for ordinary interfaces. Loopback drops the "(<name>)" -- it's
// always "lo" and the friendly name already says "(testing)" -- giving
// "Loopback (testing) — 127.0.0.1".
func label(name, ipv4 string, loopback bool) string {
	friendly := friendlyName(name)
	if loopback {
		return fmt.Sprintf("%s — %s", friendly, ipv4)
	}
	return fmt.Sprintf("%s (%s) — %s", friendly, name, ipv4)
}
