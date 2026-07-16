package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zyra/gobac/bacnet"
	"github.com/zyra/gobac/bacnet/transport"
	"github.com/zyra/gobac/bacnet/types"
	"github.com/zyra/gobac/simulator"
)

func writeScenario(t *testing.T, contents string) string {
	t.Helper()
	file, err := ioutil.TempFile("", "gobac-sim-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(contents)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func TestValidateScenario(t *testing.T) {
	path := writeScenario(t, validScenario)
	defer os.Remove(path)
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"validate", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 device(s), single-device mode") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestInspectScenario(t *testing.T) {
	path := writeScenario(t, validScenario)
	defer os.Remove(path)
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"inspect", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect returned %d: %s", code, stderr.String())
	}
	for _, expected := range []string{"Mode: single-device", "1001 Test Device", "0:1 Room Temperature"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output %q does not contain %q", stdout.String(), expected)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"unknown", "/missing/scenario.yaml"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command returned %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gobac-sim run") {
		t.Fatalf("unexpected help: %s", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runCLI([]string{"validate", "help"}, &stdout, &stderr); code != 1 {
		t.Fatalf("scenario named help returned %d", code)
	}
}

func TestEquivalentResolvedEndpointsAreRejected(t *testing.T) {
	path := writeScenario(t, `version: 1
network:
  mode: multi-port
  interface: 127.0.0.1
devices:
  - id: 1001
    port: 47808
    name: First Device
  - id: 1002
    address: 127.0.0.1
    port: 47808
    name: Second Device
`)
	defer os.Remove(path)
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"validate", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("validate returned %d", code)
	}
	if !strings.Contains(stderr.String(), "resolve to the same endpoint") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestWildcardAndConcreteEndpointsCannotSharePort(t *testing.T) {
	path := writeScenario(t, `version: 1
network:
  mode: multi-port
devices:
  - id: 1001
    address: 0.0.0.0
    port: 47808
    name: Wildcard Device
  - id: 1002
    address: 127.0.0.1
    port: 47808
    name: Concrete Device
`)
	defer os.Remove(path)
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"validate", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("validate returned %d", code)
	}
	if !strings.Contains(stderr.String(), "overlaps wildcard device") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestMultiIPRequiresConcreteAddress(t *testing.T) {
	path := writeScenario(t, `version: 1
network:
  mode: multi-ip
devices:
  - id: 1001
    address: 0.0.0.0
    port: 47808
    name: Wildcard Device
`)
	defer os.Remove(path)
	var stdout, stderr bytes.Buffer

	if code := runCLI([]string{"validate", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("validate returned %d", code)
	}
	if !strings.Contains(stderr.String(), "concrete IPv4 address") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestNetworkModeRequiresDistinctDimension(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		message  string
	}{
		{
			name: "multi-ip address",
			scenario: `version: 1
network:
  mode: multi-ip
devices:
  - {id: 1001, address: 127.0.0.1, port: 47808, name: First Device}
  - {id: 1002, address: 127.0.0.1, port: 47809, name: Second Device}
`,
			message: "share address",
		},
		{
			name: "multi-port port",
			scenario: `version: 1
network:
  mode: multi-port
devices:
  - {id: 1001, address: 127.0.0.1, port: 47808, name: First Device}
  - {id: 1002, address: 127.0.0.2, port: 47808, name: Second Device}
`,
			message: "share port",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeScenario(t, test.scenario)
			defer os.Remove(path)
			var stdout, stderr bytes.Buffer
			if code := runCLI([]string{"validate", path}, &stdout, &stderr); code != 1 {
				t.Fatalf("validate returned %d", code)
			}
			if !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("unexpected error: %s", stderr.String())
			}
		})
	}
}

func TestResolveIPv4(t *testing.T) {
	ip, err := resolveIPv4("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if got := ip.String(); got != "127.0.0.1" {
		t.Fatalf("resolved address is %s", got)
	}
	if _, err := resolveIPv4("2001:db8::1"); err == nil {
		t.Fatal("expected IPv6 address to be rejected")
	}
}

func TestServeStopsAfterCancellation(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(listener.LocalAddr().(*net.UDPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	scenario := &simulator.Scenario{
		Version: simulator.ScenarioVersion,
		Network: simulator.NetworkConfig{Mode: "single-device", Interface: "127.0.0.1", Port: port},
		Devices: []simulator.DeviceSpec{{ID: 1001, Name: "Test Device", Port: port}},
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := serve(ctx, ioutil.Discard, scenario, network); err != nil {
		t.Fatalf("serve did not stop cleanly: %v", err)
	}
}

func TestBroadcastWhoIsDiscoversEveryDevice(t *testing.T) {
	probe, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(probe.LocalAddr().(*net.UDPAddr).Port)
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	scenario := &simulator.Scenario{
		Version: simulator.ScenarioVersion,
		Network: simulator.NetworkConfig{Mode: "multi-ip", Port: port},
		Devices: []simulator.DeviceSpec{
			{ID: 1001, Address: "127.0.0.2", Port: port, Name: "First Device"},
			{ID: 1002, Address: "127.0.0.3", Port: port, Name: "Second Device"},
		},
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ioutil.Discard, scenario, network) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("simulator shutdown: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("simulator did not stop")
		}
	}()

	client, err := transport.ListenUDP(transport.NewEndpoint(net.IPv4zero, port))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	whoIs := bacnet.NewRequest()
	whoIs.Header.Function = types.BvlcFunctionOriginalBroadcastNpdu
	whoIs.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	whoIs.Apdu.ServiceChoice = types.UnconfirmedServiceWhoIs
	payload, err := whoIs.MarshalBinary()
	whoIs.Release()
	if err != nil {
		t.Fatal(err)
	}

	sendDone := make(chan struct{})
	defer close(sendDone)
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			_ = client.Write(ctx, transport.NewEndpoint(net.IPv4(127, 255, 255, 255), port), payload)
			select {
			case <-ticker.C:
			case <-sendDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	readContext, stopReading := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopReading()
	discovered := make(map[uint32]bool)
	for len(discovered) < len(scenario.Devices) {
		datagram, err := client.Read(readContext)
		if err != nil {
			t.Fatalf("read I-Am responses after discovering %v: %v", discovered, err)
		}
		packet, err := bacnet.ParseRequest(datagram.Payload, datagram.Source.UDPAddr())
		if err != nil {
			continue
		}
		if packet.Apdu.PduType == types.PduTypeUnconfirmedServiceRequest && packet.Apdu.ServiceChoice == types.UnconfirmedServiceIAm {
			if device, ok := packet.Apdu.ResponseData.(*types.Device); ok {
				discovered[device.DeviceInstance] = true
			}
		}
		packet.Release()
	}
	if !discovered[1001] || !discovered[1002] {
		t.Fatalf("discovered devices %v", discovered)
	}
}

const validScenario = `version: 1
network:
  mode: single-device
  interface: 127.0.0.1
  port: 47808
devices:
  - id: 1001
    name: Test Device
    objects:
      - type: analog-input
        instance: 1
        name: Room Temperature
        present_value: 22.5
`
