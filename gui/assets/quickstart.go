// Package assets embeds static resources bundled into the GoBAC Workstation
// binary.
package assets

import _ "embed"

// QuickstartScenario is the bundled demo scenario (YAML) run in-process by
// the Quickstart view (gui-architecture.md §4.5, task G8): three devices,
// each on its own loopback UDP port, so a one-click run never touches
// BACnet/IP's well-known port 47808 or conflicts with a real network.
//
//go:embed quickstart.yaml
var QuickstartScenario []byte
