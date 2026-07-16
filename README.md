# GoBAC

GoBAC is a Go implementation of the BACnet/IP protocol. It provides a client-side library for discovering BACnet devices, reading and writing object properties, and subscribing to change-of-value notifications. A command-line client is included for discovery and common property operations.

The project is being refreshed with an emphasis on protocol fixtures, interoperability testing, and backwards-compatible improvements. The supported protocol surface is intentionally described below so applications do not have to infer it from the source.

## Supported functionality

- BACnet/IP over IPv4 using original unicast and broadcast BVLC messages
- Who-Is broadcasts and I-Am response decoding
- Confirmed ReadProperty and WriteProperty requests
- SubscribeCOV requests and confirmed COV notification handling
- Simple ACK, Complex ACK, Error, Reject, and Abort response decoding
- Device and object helpers for enumerating object lists and properties
- Encoding and decoding for the BACnet application data types currently represented in `bacnet/types`

## Current limitations

- The library is currently a BACnet client. It does not expose a general server-side service handler API.
- BBMD operation, foreign-device registration, and forwarded NPDU handling are not implemented.
- Routed BACnet networks, BACnet/IPv6, BACnet/SC, and MS/TP are not implemented.
- Segmented APDUs and request retries are not implemented.
- ReadPropertyMultiple and WritePropertyMultiple are not implemented; the object helpers issue individual ReadProperty requests.
- The API identifies remote devices by IPv4 address and assumes the configured BACnet UDP port.
- The CLI's `scan` command is an exploratory helper and can generate many individual requests on a large device.

## Building

The repository vendors its dependencies.

```sh
go test -mod=vendor ./...
go build -mod=vendor -o gobac ./cmd/gobac
```

The Docker image builds the command-line client as its entry point:

```sh
docker build -t gobac .
```

## Command-line client

Select the network interface that is connected to the BACnet/IP network. The standard BACnet/IP UDP port is `47808` (`0xBAC0`), which is the default for both port options.

```sh
# Discover devices for three seconds
./gobac --interface eth0 whois --duration 3

# Read Present_Value (property 85) from analog-value object instance 1
./gobac --interface eth0 readprop 192.0.2.10 2 1 85

# Write a BACnet REAL (application tag 4) at priority 9
./gobac --interface eth0 writeprop --priority 9 192.0.2.10 2 1 85 4 21.5
```

Run `./gobac help` or `./gobac <command> --help` for all options. The CLI accepts numeric BACnet object types, property identifiers, and application tags.

## Library example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zyra/gobac/bacnet"
)

func main() {
	config := bacnet.NewServerConfig().
		SetInterfaceName("eth0").
		SetDefaultTimeout(5 * time.Second)

	client, err := bacnet.NewServer(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer client.Shutdown()

	go client.Listen(ctx)
	<-client.Start()

	devices, err := client.WhoIs(ctx, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	for device := range devices {
		fmt.Printf("device %d at %s\n", device.ObjectId.Instance, device.IPAddress)
	}
}
```

Despite the historical `Server` name, the type currently acts as a BACnet/IP client and response dispatcher.

## Development

Changes to protocol encoding or decoding should include packet fixtures or focused unit tests. For interoperability work, compare emitted packets and responses with a separate BACnet implementation such as [bacnet-stack](https://github.com/bacnet-stack/bacnet-stack).

Before submitting a change, run:

```sh
gofmt -w <changed-go-files>
go test -mod=vendor ./...
go vet -mod=vendor ./...
go build -mod=vendor ./cmd/gobac
```
