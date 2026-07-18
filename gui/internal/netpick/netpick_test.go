package netpick

import (
	"errors"
	"net"
	"testing"
)

// realLoopback returns the host's real loopback interface. Every supported
// test environment (Linux CI, dev boxes) has one up with an IPv4 address,
// so tests use it as a genuine, resolvable Candidates() input instead of a
// synthetic net.Interface whose Addrs() would have nothing real to query.
func realLoopback(t *testing.T) net.Interface {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("net.Interfaces(): %v", err)
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 && ifc.Flags&net.FlagUp != 0 {
			return ifc
		}
	}
	t.Skip("no up loopback interface on this host")
	return net.Interface{}
}

func TestCandidatesExcludesDownInterfaces(t *testing.T) {
	lo := realLoopback(t)
	down := net.Interface{Name: "down0", Flags: 0}

	got := Candidates(func() ([]net.Interface, error) {
		return []net.Interface{lo, down}, nil
	})

	if len(got) != 1 || got[0].Name != lo.Name {
		t.Fatalf("Candidates = %+v, want only %q", got, lo.Name)
	}
}

func TestCandidatesExcludesUpInterfacesWithoutIPv4(t *testing.T) {
	lo := realLoopback(t)
	// A nonexistent interface: Up but not resolvable to any address, so
	// ifc.Addrs() fails and it must be excluded.
	noAddr := net.Interface{Name: "nonexistent999", Index: 999999, Flags: net.FlagUp}

	got := Candidates(func() ([]net.Interface, error) {
		return []net.Interface{lo, noAddr}, nil
	})

	if len(got) != 1 || got[0].Name != lo.Name {
		t.Fatalf("Candidates = %+v, want only %q", got, lo.Name)
	}
}

func TestCandidatesReturnsNilOnListError(t *testing.T) {
	got := Candidates(func() ([]net.Interface, error) {
		return nil, errors.New("boom")
	})
	if got != nil {
		t.Errorf("Candidates = %+v, want nil", got)
	}
}

func TestCandidatesLabelsLoopback(t *testing.T) {
	lo := realLoopback(t)

	got := Candidates(func() ([]net.Interface, error) {
		return []net.Interface{lo}, nil
	})

	if len(got) != 1 {
		t.Fatalf("Candidates = %+v, want exactly one candidate", got)
	}
	c := got[0]
	if !c.Loopback {
		t.Errorf("Candidate.Loopback = false, want true")
	}
	want := "Loopback (testing) — " + c.IPv4
	if c.Label != want {
		t.Errorf("Candidate.Label = %q, want %q", c.Label, want)
	}
	if c.IPv4 == "" {
		t.Errorf("Candidate.IPv4 = empty, want a dotted IPv4 address")
	}
}

func TestSortCandidatesOrdersNonLoopbackFirstThenName(t *testing.T) {
	cands := []Candidate{
		{Name: "lo", Loopback: true},
		{Name: "wlan0", Loopback: false},
		{Name: "eno0", Loopback: false},
	}

	sortCandidates(cands)

	want := []string{"eno0", "wlan0", "lo"}
	for i, name := range want {
		if cands[i].Name != name {
			t.Fatalf("sortCandidates order = %v, want %v", namesOf(cands), want)
		}
	}
}

func namesOf(cands []Candidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.Name
	}
	return out
}

func TestLabelFormatsNonLoopback(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"wlan0", "Wi-Fi (wlan0) — 10.0.0.5"},
		{"en0", "Wired (en0) — 10.0.0.5"},
		{"eth0", "Wired (eth0) — 10.0.0.5"},
		{"tun0", "Network (tun0) — 10.0.0.5"},
	}
	for _, tc := range cases {
		got := label(tc.name, "10.0.0.5", false)
		if got != tc.want {
			t.Errorf("label(%q, ...) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLabelLoopbackOmitsName(t *testing.T) {
	got := label("lo", "127.0.0.1", true)
	want := "Loopback (testing) — 127.0.0.1"
	if got != want {
		t.Errorf("label(loopback) = %q, want %q", got, want)
	}
}

func TestAutomaticPrefersFirstNonLoopback(t *testing.T) {
	cands := []Candidate{
		{Name: "lo", Loopback: true},
		{Name: "eno0", Loopback: false},
		{Name: "wlan0", Loopback: false},
	}

	got, ok := Automatic(cands)
	if !ok {
		t.Fatal("Automatic returned ok = false, want true")
	}
	if got.Name != "eno0" {
		t.Errorf("Automatic = %+v, want eno0", got)
	}
}

func TestAutomaticFallsBackToLoopback(t *testing.T) {
	cands := []Candidate{
		{Name: "lo", Loopback: true},
	}

	got, ok := Automatic(cands)
	if !ok {
		t.Fatal("Automatic returned ok = false, want true")
	}
	if got.Name != "lo" {
		t.Errorf("Automatic = %+v, want lo", got)
	}
}

func TestAutomaticReturnsFalseWhenEmpty(t *testing.T) {
	_, ok := Automatic(nil)
	if ok {
		t.Error("Automatic(nil) ok = true, want false")
	}
}

func TestIsVirtualMatchesCommonVirtualPrefixes(t *testing.T) {
	virtual := []string{
		"docker0", "br-4a3b2c1d", "veth1234", "virbr0",
		"cni0", "flannel.1", "tailscale0", "tun0", "tap0", "wg0", "zt7nnq",
	}
	for _, name := range virtual {
		if !isVirtual(name) {
			t.Errorf("isVirtual(%q) = false, want true", name)
		}
	}

	physical := []string{"eno0", "eth0", "wlan0", "en0", "lo"}
	for _, name := range physical {
		if isVirtual(name) {
			t.Errorf("isVirtual(%q) = true, want false", name)
		}
	}
}

// TestSortCandidatesRanksPhysicalBeforeVirtual covers finding 2: a docker
// bridge sorts alphabetically before most real NIC names ("br-..." <
// "eno0"), so without ranking by Virtual it would beat the real LAN
// interface for both display order and Automatic's pick. Physical
// interfaces must sort first among non-loopback candidates regardless of
// name.
func TestSortCandidatesRanksPhysicalBeforeVirtual(t *testing.T) {
	cands := []Candidate{
		{Name: "br-4a3b2c1d", Virtual: true},
		{Name: "docker0", Virtual: true},
		{Name: "eno0"},
		{Name: "lo", Loopback: true},
	}

	sortCandidates(cands)

	want := []string{"eno0", "br-4a3b2c1d", "docker0", "lo"}
	if got := namesOf(cands); !equalStrings(got, want) {
		t.Fatalf("sortCandidates order = %v, want %v", got, want)
	}
}

// TestAutomaticPrefersPhysicalOverVirtual covers finding 2's main claim:
// with both a docker bridge and a real NIC present, Automatic must resolve
// to the physical candidate even when the virtual one sorts first
// alphabetically.
func TestAutomaticPrefersPhysicalOverVirtual(t *testing.T) {
	cands := []Candidate{
		{Name: "br-4a3b2c1d", Virtual: true},
		{Name: "docker0", Virtual: true},
		{Name: "eno0"},
	}
	sortCandidates(cands)

	got, ok := Automatic(cands)
	if !ok {
		t.Fatal("Automatic returned ok = false, want true")
	}
	if got.Name != "eno0" {
		t.Errorf("Automatic = %+v, want eno0", got)
	}
}

// TestAutomaticFallsBackToVirtualWhenOnlyVirtualPresent covers a
// docker-only host: no physical NIC exists, so Automatic must still
// resolve to the (virtual) non-loopback candidate rather than skipping
// straight to loopback or failing.
func TestAutomaticFallsBackToVirtualWhenOnlyVirtualPresent(t *testing.T) {
	cands := []Candidate{
		{Name: "lo", Loopback: true},
		{Name: "docker0", Virtual: true},
	}

	got, ok := Automatic(cands)
	if !ok {
		t.Fatal("Automatic returned ok = false, want true")
	}
	if got.Name != "docker0" {
		t.Errorf("Automatic = %+v, want docker0", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
