# Compatibility and roadmap

This comparison uses bacnet-stack commit `73584d6` as the interoperability reference. It is a direction-setting gap analysis, not a claim of BTL conformance.

## Current GoBAC surface

GoBAC provides BACnet/IP/IPv4 discovery, ReadProperty, WriteProperty, SubscribeCOV, response decoding, device/object helpers, a server-side responder, pluggable transports, and a scenario-driven simulator. The wire layer covers the application values represented in `bacnet/types`, BVLC original unicast/broadcast framing, local and routed NPCI address fields, and the common APDU acknowledgment/error forms.

The simulator adds device-side Who-Is/I-Am, ReadProperty, WriteProperty, ReadPropertyMultiple, and SubscribeCOV behavior. These paths have byte fixtures and end-to-end tests against the in-memory network.

## Main bacnet-stack gaps

| Area | bacnet-stack capability not yet present in GoBAC |
| --- | --- |
| BACnet/IP infrastructure | BBMD tables, foreign-device registration, forwarded NPDUs, and full router/network-message operation |
| Other data links | BACnet/IPv6, BACnet/SC, MS/TP, Ethernet, and ARCNET |
| APDU reliability | Segmentation and reassembly, SegmentACK handling, transaction retries, and confirmed-notification retransmission |
| Data access | Client ReadPropertyMultiple, WritePropertyMultiple, ReadRange, list-element services, and file services |
| Discovery and time | Who-Has/I-Have, Who-Am-I/You-Are, TimeSynchronization, and UTCTimeSynchronization |
| Events and alarms | Event notification, alarm acknowledgment, alarm/event summaries, and event enrollment behavior |
| Device management | DeviceCommunicationControl, ReinitializeDevice, CreateObject, DeleteObject, and private transfer |
| COV breadth | SubscribeCOVProperty, multiple-COV services, per-property increments, and full subscription retry state |
| Object model | bacnet-stack's broad standard object catalog, property lists, persistence hooks, and device profiles |
| Qualification | BTL test coverage, interoperability captures, fuzzing, and long-running network stress suites |

## Recommended order

1. Build packet-capture interoperability tests that run GoBAC against bacnet-stack for discovery, RP/WP/RPM, errors, COV, routed NPCI fields, and malformed input.
2. Add a transaction manager with retries, duplicate detection, and APDU segmentation/reassembly before broadening high-volume services.
3. Implement BBMD/foreign-device support and forwarded NPDU decoding; this unlocks realistic multi-subnet BACnet/IP deployments.
4. Add client ReadPropertyMultiple and use it in object enumeration to reduce network traffic.
5. Expand simulator control through a small local API so tests can change values, advance time, inject faults, and inspect subscriptions deterministically.
6. Add fuzz targets for every decoder and a corpus built from bacnet-stack fixtures and sanitized real captures.
7. Add BACnet/IPv6, followed by BACnet/SC. Treat MS/TP as a separate transport milestone because timing and token behavior need hardware-aware tests.

Every protocol milestone should keep exact packet fixtures, a bacnet-stack cross-check, and an independent verification pass as merge requirements.
