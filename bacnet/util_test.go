package bacnet

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestGetNetworkSetEmptyNameReturnsWildcard verifies that requesting the
// explicit wildcard ("all networks") interface name succeeds and reports the
// wildcard source paired with the limited broadcast destination, rather than
// failing the way any other unresolvable name does.
func TestGetNetworkSetEmptyNameReturnsWildcard(t *testing.T) {
	ns, err := getNetworkSet("")
	if err != nil {
		t.Fatalf("getNetworkSet(\"\") returned error: %v", err)
	}
	if ns == nil {
		t.Fatal("getNetworkSet(\"\") returned nil networkSet")
	}
	if ns.InterfaceName != "" {
		t.Errorf("InterfaceName = %q, want empty", ns.InterfaceName)
	}
	if ns.Interface != nil {
		t.Errorf("Interface = %+v, want nil", ns.Interface)
	}
	if !ns.IPv4.Equal(net.IPv4zero) {
		t.Errorf("IPv4 = %v, want %v", ns.IPv4, net.IPv4zero)
	}
	if !ns.BroadcastIPv4.Equal(net.IPv4bcast) {
		t.Errorf("BroadcastIPv4 = %v, want %v", ns.BroadcastIPv4, net.IPv4bcast)
	}
}

// TestGetNetworkSetUnknownNameStillErrors is a regression check that a
// non-empty, unresolvable interface name is unaffected by the wildcard
// carve-out and still fails exactly as before.
func TestGetNetworkSetUnknownNameStillErrors(t *testing.T) {
	if _, err := getNetworkSet("not-a-real-interface-xyz"); err == nil {
		t.Fatal("getNetworkSet with an unknown name returned nil error, want an error")
	}
}

// TestGetNetworkSetNamedInterfaceStillResolves is a regression check that a
// real, existing interface name (loopback is present on every test host)
// still resolves through the normal net.InterfaceByName path, unaffected by
// the new empty-name branch.
func TestGetNetworkSetNamedInterfaceStillResolves(t *testing.T) {
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("net.Interfaces: %v", err)
	}
	var loName string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			loName = ifc.Name
			break
		}
	}
	if loName == "" {
		t.Skip("no loopback interface found on this host")
	}

	ns, err := getNetworkSet(loName)
	if err != nil {
		t.Fatalf("getNetworkSet(%q): %v", loName, err)
	}
	if ns.InterfaceName != loName {
		t.Errorf("InterfaceName = %q, want %q", ns.InterfaceName, loName)
	}
	if ns.Interface == nil {
		t.Error("Interface = nil, want the resolved *net.Interface")
	}
}

// TestListenContextWildcardBindsOnEphemeralPort is a loopback smoke test:
// with InterfaceName left empty, ListenContext should bind the wildcard
// address on an OS-assigned ephemeral port and report itself listening,
// exactly as a named-interface config does. It never touches a real
// network -- the port is 0 (ephemeral) and traffic stays local.
func TestListenContextWildcardBindsOnEphemeralPort(t *testing.T) {
	cfg := NewServerConfig()
	cfg.InterfaceName = ""
	cfg.ServerBBMDPort = 0
	cfg.ListenPort = 0

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	result := make(chan error, 1)
	go func() { result <- s.ListenContext(ctx) }()

	select {
	case <-s.Start():
	case <-time.After(time.Second):
		cancel()
		t.Fatal("listener did not start")
	}
	if !s.networkSet.IPv4.Equal(net.IPv4zero) {
		t.Errorf("resolved IPv4 = %v, want %v", s.networkSet.IPv4, net.IPv4zero)
	}

	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not stop")
	}
	s.Shutdown()
}
