package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/zyra/gobac/bacnet"
	"github.com/zyra/gobac/bacnet/responder"
	"github.com/zyra/gobac/bacnet/transport"
	"github.com/zyra/gobac/bacnet/types"
	"github.com/zyra/gobac/simulator"
)

const usage = `Usage:
  gobac-sim run <scenario>
  gobac-sim validate <scenario>
  gobac-sim inspect <scenario>`

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "run", "validate", "inspect":
	default:
		fmt.Fprintf(stderr, "gobac-sim: unknown command %q\n%s\n", args[0], usage)
		return 2
	}

	scenario, network, err := loadScenario(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "gobac-sim: %v\n", err)
		return 1
	}

	switch args[0] {
	case "validate":
		fmt.Fprintf(stdout, "Scenario is valid: %d device(s), %s mode\n", len(network.Devices), scenario.Network.Mode)
		return 0
	case "inspect":
		if err := inspectScenario(stdout, scenario, network); err != nil {
			fmt.Fprintf(stderr, "gobac-sim: %v\n", err)
			return 1
		}
		return 0
	case "run":
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		go func() {
			select {
			case <-signals:
				cancel()
			case <-ctx.Done():
			}
		}()

		if err := serve(ctx, stdout, scenario, network); err != nil {
			fmt.Fprintf(stderr, "gobac-sim: %v\n", err)
			return 1
		}
		return 0
	}
	return 2
}

func loadScenario(path string) (*simulator.Scenario, *simulator.Network, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open scenario: %v", err)
	}
	defer file.Close()

	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	scenario, err := simulator.DecodeScenario(file, format)
	if err != nil {
		return nil, nil, fmt.Errorf("decode scenario: %v", err)
	}
	network, err := scenario.BuildNetwork()
	if err != nil {
		return nil, nil, fmt.Errorf("build scenario: %v", err)
	}
	if err := validateResolvedEndpoints(scenario, network); err != nil {
		return nil, nil, err
	}
	return scenario, network, nil
}

func validateResolvedEndpoints(scenario *simulator.Scenario, network *simulator.Network) error {
	endpoints := make(map[string]uint32, len(network.Devices))
	wildcards := make(map[uint16]uint32)
	concrete := make(map[uint16]uint32)
	modeAddresses := make(map[string]uint32)
	modePorts := make(map[uint16]uint32)
	for _, spec := range scenario.Devices {
		device, err := network.Device(spec.ID)
		if err != nil {
			return fmt.Errorf("device %d: %v", spec.ID, err)
		}
		endpoint, err := deviceEndpoint(scenario, device)
		if err != nil {
			return fmt.Errorf("device %d endpoint: %v", device.ID, err)
		}
		key := endpoint.String()
		if previous, exists := endpoints[key]; exists {
			return fmt.Errorf("devices %d and %d resolve to the same endpoint %s", previous, device.ID, key)
		}
		if isWildcardIPv4(endpoint.IP) {
			if scenario.Network.Mode == "multi-ip" {
				return fmt.Errorf("device %d must use a concrete IPv4 address in multi-ip mode", device.ID)
			}
			if previous, exists := concrete[endpoint.Port]; exists {
				return fmt.Errorf("wildcard device %d overlaps device %d on port %d", device.ID, previous, endpoint.Port)
			}
			wildcards[endpoint.Port] = device.ID
		} else {
			if previous, exists := wildcards[endpoint.Port]; exists {
				return fmt.Errorf("device %d overlaps wildcard device %d on port %d", device.ID, previous, endpoint.Port)
			}
			concrete[endpoint.Port] = device.ID
		}
		if scenario.Network.Mode == "multi-ip" {
			address := endpoint.IP.String()
			if previous, exists := modeAddresses[address]; exists {
				return fmt.Errorf("devices %d and %d share address %s in multi-ip mode", previous, device.ID, address)
			}
			modeAddresses[address] = device.ID
		}
		if scenario.Network.Mode == "multi-port" {
			if previous, exists := modePorts[endpoint.Port]; exists {
				return fmt.Errorf("devices %d and %d share port %d in multi-port mode", previous, device.ID, endpoint.Port)
			}
			modePorts[endpoint.Port] = device.ID
		}
		endpoints[key] = device.ID
	}
	return nil
}

func inspectScenario(writer io.Writer, scenario *simulator.Scenario, network *simulator.Network) error {
	fmt.Fprintf(writer, "Mode: %s\n", scenario.Network.Mode)
	fmt.Fprintf(writer, "Devices: %d\n", len(network.Devices))

	deviceIDs := make([]int, 0, len(network.Devices))
	for id := range network.Devices {
		deviceIDs = append(deviceIDs, int(id))
	}
	sort.Ints(deviceIDs)
	for _, rawID := range deviceIDs {
		device, err := network.Device(uint32(rawID))
		if err != nil {
			return err
		}
		endpoint, err := deviceEndpoint(scenario, device)
		if err != nil {
			return fmt.Errorf("device %d: %v", device.ID, err)
		}
		fmt.Fprintf(writer, "- %d %s at %s\n", device.ID, device.Name, endpoint.String())

		objects := make([]simulator.ObjectID, 0, len(device.Objects))
		for id := range device.Objects {
			objects = append(objects, id)
		}
		sort.Slice(objects, func(i, j int) bool {
			if objects[i].Type == objects[j].Type {
				return objects[i].Instance < objects[j].Instance
			}
			return objects[i].Type < objects[j].Type
		})
		for _, id := range objects {
			object := device.Objects[id]
			fmt.Fprintf(writer, "  - %s %s (%d properties)\n", id, object.Name, len(object.Properties))
		}
	}
	return nil
}

type runningDevice struct {
	id     uint32
	server *responder.Server
	conn   transport.Conn
}

type serveResult struct {
	id        uint32
	broadcast bool
	err       error
}

type broadcastListener struct {
	conn           transport.Conn
	deviceIndexes  []int
	wildcardDevice int
}

func serve(ctx context.Context, writer io.Writer, scenario *simulator.Scenario, network *simulator.Network) error {
	running := make([]runningDevice, 0, len(scenario.Devices))
	for _, spec := range scenario.Devices {
		device, err := network.Device(spec.ID)
		if err != nil {
			closeDevices(running)
			return fmt.Errorf("device %d: %v", spec.ID, err)
		}
		endpoint, err := deviceEndpoint(scenario, device)
		if err != nil {
			closeDevices(running)
			return fmt.Errorf("device %d: %v", device.ID, err)
		}
		conn, err := transport.ListenUDP(endpoint)
		if err != nil {
			closeDevices(running)
			return fmt.Errorf("listen for device %d on %s: %v", device.ID, endpoint, err)
		}
		server := responder.NewServer()
		simulator.NewApplication(device, simulator.RealClock{}).Register(server)
		running = append(running, runningDevice{id: device.ID, server: server, conn: conn})
		fmt.Fprintf(writer, "Device %d listening on %s\n", device.ID, conn.LocalEndpoint())
	}
	broadcasts, err := startBroadcastListeners(running)
	if err != nil {
		closeDevices(running)
		return err
	}
	defer closeBroadcastListeners(broadcasts)

	direct := make([]bool, len(running))
	for i := range direct {
		direct[i] = true
	}
	for _, listener := range broadcasts {
		if listener.wildcardDevice >= 0 {
			direct[listener.wildcardDevice] = false
		}
	}
	workerCount := len(broadcasts)
	for _, enabled := range direct {
		if enabled {
			workerCount++
		}
	}
	results := make(chan serveResult, workerCount)
	for i := range running {
		if !direct[i] {
			continue
		}
		go func(device runningDevice) {
			results <- serveResult{id: device.id, err: serveDevice(ctx, device)}
		}(running[i])
	}
	for i := range broadcasts {
		go func(listener broadcastListener) {
			results <- serveResult{broadcast: true, err: serveBroadcasts(ctx, listener, running)}
		}(broadcasts[i])
	}

	var result serveResult
	select {
	case <-ctx.Done():
		closeDevices(running)
		closeBroadcastListeners(broadcasts)
		for i := 0; i < workerCount; i++ {
			<-results
		}
		return nil
	case result = <-results:
		closeDevices(running)
		closeBroadcastListeners(broadcasts)
		for i := 1; i < workerCount; i++ {
			<-results
		}
		if ctx.Err() != nil {
			return nil
		}
		if result.broadcast {
			return fmt.Errorf("broadcast listener stopped: %v", result.err)
		}
		return fmt.Errorf("device %d stopped: %v", result.id, result.err)
	}
}

func serveDevice(ctx context.Context, device runningDevice) error {
	ignoreBroadcast := !isWildcardIPv4(device.conn.LocalEndpoint().IP)
	for {
		datagram, err := device.conn.Read(ctx)
		if err != nil {
			return err
		}
		if ignoreBroadcast && isOriginalBroadcast(datagram) {
			// The wildcard listener dispatches one copy to every device on
			// this port. Some operating systems also deliver a copy to the
			// concrete-IP socket.
			continue
		}
		_ = device.server.ServeDatagram(ctx, device.conn, datagram)
	}
}

func startBroadcastListeners(devices []runningDevice) ([]broadcastListener, error) {
	byPort := make(map[uint16][]int)
	for i := range devices {
		endpoint := devices[i].conn.LocalEndpoint()
		byPort[endpoint.Port] = append(byPort[endpoint.Port], i)
	}
	ports := make([]int, 0, len(byPort))
	for port := range byPort {
		ports = append(ports, int(port))
	}
	sort.Ints(ports)

	listeners := make([]broadcastListener, 0, len(ports))
	for _, rawPort := range ports {
		indexes := byPort[uint16(rawPort)]
		wildcard := -1
		hasConcrete := false
		for _, index := range indexes {
			if isWildcardIPv4(devices[index].conn.LocalEndpoint().IP) {
				wildcard = index
			} else {
				hasConcrete = true
			}
		}
		if !hasConcrete {
			continue
		}

		var conn transport.Conn
		if wildcard >= 0 {
			conn = devices[wildcard].conn
		} else {
			var err error
			conn, err = transport.ListenUDP(transport.NewEndpoint(net.IPv4zero, uint16(rawPort)))
			if err != nil {
				closeBroadcastListeners(listeners)
				return nil, fmt.Errorf("listen for broadcasts on port %d: %v", rawPort, err)
			}
		}
		listeners = append(listeners, broadcastListener{
			conn:           conn,
			deviceIndexes:  append([]int(nil), indexes...),
			wildcardDevice: wildcard,
		})
	}
	return listeners, nil
}

func serveBroadcasts(ctx context.Context, listener broadcastListener, devices []runningDevice) error {
	for {
		datagram, err := listener.conn.Read(ctx)
		if err != nil {
			return err
		}
		if isOriginalBroadcast(datagram) {
			for _, index := range listener.deviceIndexes {
				copy := datagram
				copy.Destination = devices[index].conn.LocalEndpoint()
				_ = devices[index].server.ServeDatagram(ctx, devices[index].conn, copy)
			}
			continue
		}
		if listener.wildcardDevice >= 0 {
			index := listener.wildcardDevice
			_ = devices[index].server.ServeDatagram(ctx, devices[index].conn, datagram)
		}
	}
}

func isOriginalBroadcast(datagram transport.Datagram) bool {
	packet, err := bacnet.ParseRequest(datagram.Payload, datagram.Source.UDPAddr())
	if err != nil {
		return false
	}
	defer packet.Release()
	return packet.Header.Function == types.BvlcFunctionOriginalBroadcastNpdu
}

func isWildcardIPv4(ip net.IP) bool {
	value := ip.To4()
	return value != nil && value[0] == 0 && value[1] == 0 && value[2] == 0 && value[3] == 0
}

func closeDevices(devices []runningDevice) {
	for i := range devices {
		_ = devices[i].conn.Close()
	}
}

func closeBroadcastListeners(listeners []broadcastListener) {
	for i := range listeners {
		_ = listeners[i].conn.Close()
	}
}

func deviceEndpoint(scenario *simulator.Scenario, device *simulator.Device) (transport.Endpoint, error) {
	address := strings.TrimSpace(device.Address)
	if address == "" {
		address = strings.TrimSpace(scenario.Network.Interface)
	}
	ip, err := resolveIPv4(address)
	if err != nil {
		return transport.Endpoint{}, err
	}
	return transport.NewEndpoint(ip, device.Port), nil
}

func resolveIPv4(addressOrInterface string) (net.IP, error) {
	if addressOrInterface == "" {
		return net.IPv4zero, nil
	}
	if ip := net.ParseIP(addressOrInterface); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4, nil
		}
		return nil, fmt.Errorf("%q is not an IPv4 address", addressOrInterface)
	}

	iface, err := net.InterfaceByName(addressOrInterface)
	if err != nil {
		return nil, fmt.Errorf("find interface %q: %v", addressOrInterface, err)
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("list addresses for interface %q: %v", addressOrInterface, err)
	}
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}
	return nil, fmt.Errorf("interface %q has no IPv4 address", addressOrInterface)
}
