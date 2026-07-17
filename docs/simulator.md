# BACnet network simulator

`gobac-sim` runs one or more BACnet/IP devices from a strict YAML or JSON scenario. It uses the same wire codecs, responder, and transport packages as the library.

## Commands

```sh
gobac-sim validate scenario.yaml
gobac-sim inspect scenario.yaml
gobac-sim run scenario.yaml
```

`validate` checks schema, object identifiers, endpoint conflicts, and supported object types. `inspect` prints the devices, endpoints, and objects without opening sockets. `run` continues until SIGINT or SIGTERM.

The repository includes [an example scenario](../examples/simulator.yaml).

## Network modes

- `single-device` requires exactly one device.
- `multi-ip` assigns each device a distinct local IPv4 address. The addresses must already exist on the host.
- `multi-port` assigns devices distinct UDP ports, which is useful for isolated development on one host.

`network.interface` accepts either an IPv4 address or an interface name. A device-level `address` or `port` overrides the network default.

Broadcast behavior depends on the host network configuration. Multi-IP scenarios should use addresses on the selected BACnet interface; container and loopback networks may need explicit broadcast routing.

## Implemented behavior

- Who-Is range filtering and broadcast I-Am replies
- Who-Has range filtering, matching by object identifier or exact object name, and broadcast I-Have replies
- ReadProperty, including array index zero, individual elements, and all elements
- WriteProperty with writable checks and command priorities 1 through 16, excluding reserved priority 6
- WritePropertyMultiple with atomic validate-then-apply semantics and the standard WritePropertyMultiple-Error on failure
- ReadPropertyMultiple with per-property values or errors and the `ALL` selector
- SubscribeCOV registration, renewal, cancellation, expiry, initial notification, and notification after successful writes
- TimeSynchronization and UTCTimeSynchronization, storing the received date and time onto the device's Local_Date and Local_Time properties (readable only after a sync; the simulator has no UTC-offset model, so UTCTimeSynchronization values are stored exactly as received)
- Device, analog, binary, and multi-state object scenarios
- Deterministic manual clocks for subscription tests

Confirmed COV notifications use valid confirmed-request APDUs and invoke identifiers. The initial simulator does not retransmit notifications when an acknowledgment is missing.

For a commandable object, `relinquish_default` is the fallback value. An explicit `present_value` starts as a command at `initial_priority` (default 16) so the configured initial state is observable and can later be relinquished.

## Current constraints

- BACnet/IP over IPv4 only
- No BBMD, foreign-device, router, BACnet/IPv6, BACnet/SC, or MS/TP simulation
- No segmented APDUs
- No persistence or external control API while a scenario is running
- Scenarios are deterministic; there is no randomness and no `seed` field
- COV notifications currently track Present_Value writes; COV-property subscriptions and multi-property COV are not implemented
- The scenario object catalog is intentionally smaller than the BACnet object catalog
