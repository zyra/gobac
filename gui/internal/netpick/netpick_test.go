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
