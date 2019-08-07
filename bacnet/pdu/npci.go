package pdu

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
	"net"
)

type Npci struct {
	ProtocolVersion     uint8
	NetworkLayerMessage bool
	IsConfirmed         bool
	ExpectingReply      bool
	Priority            types.MessagePriority
	HopCount            uint8
	ControlOctet        uint8
	Length              int

	DestinationNet    types.Uint16
	DestinationLength uint8
	DestinationMAC    *net.HardwareAddr
	DestinationIP     *net.IP

	SourceNet    types.Uint16
	SourceLength uint8
	SourceMAC    *net.HardwareAddr
	SourceIP     *net.IP
}

func (n *Npci) Reset() {
	n.ProtocolVersion = types.BACnetVersion
	n.NetworkLayerMessage = false
	n.IsConfirmed = false
	n.ExpectingReply = false
	n.Priority = types.MessagePriorityNormal
	n.HopCount = 255
	n.ControlOctet = 0
	n.Length = 0
	n.DestinationNet = 0
	n.DestinationLength = 0
	n.DestinationMAC = nil
	n.DestinationIP = nil
	n.SourceNet = 0
	n.SourceLength = 0
	n.SourceMAC = nil
	n.SourceIP = nil
}

func (n *Npci) MarshalBinary() (b []byte, e error) {
	if n.NetworkLayerMessage {
		return nil, errors.New("network layer messages are not supported")
	}

	if n.IsConfirmed {
		b = make([]byte, 2)
	} else {
		b = make([]byte, 6)
	}

	b[0] = byte(n.ProtocolVersion)
	b[1] = 0

	if n.NetworkLayerMessage {
		b[1] |= types.BIT7
	}

	if n.DestinationNet > 0 {
		b[1] |= types.BIT5

		// Mark message as broadcast
		if v, e := types.Uint16(65535).MarshalBinary(); e != nil {
			return nil, e
		} else {
			copy(b[2:], v)
		}

		// Destination length
		b[4] = 0

		// Hop count
		b[5] = byte(n.HopCount)
	}

	if n.ExpectingReply {
		b[1] |= types.BIT2
	}

	b[1] |= byte(n.Priority) & types.BITS01

	return b, e
}

func (n *Npci) UnmarshalBinary(b []byte) error {
	buff := bytes.NewBuffer(b)

	if b, e := buff.ReadByte(); e != nil {
		return e
	} else {
		n.ProtocolVersion = b
	}

	if n.ProtocolVersion != types.BACnetVersion {
		return fmt.Errorf("expected protocol version to be %d but got %d", 1, n.ProtocolVersion)
	}

	if b, e := buff.ReadByte(); e != nil {
		return e
	} else {
		n.ControlOctet = b
	}

	if n.NetworkLayerMessage = n.ControlOctet&types.BIT7 != 0; n.NetworkLayerMessage {
		return fmt.Errorf("network layer messages aren't supported")
	}

	if hasDest := n.ControlOctet & types.BIT5; hasDest != 0 {
		// DNET, DLEN, and DADR are present
		// We don't need this info since we're not dealing with raw packets
		// Let's just shave a few bytes off the buffer
		// DNET is 2 octets
		// DLEN is 1 octet
		// DADR is DLEN
		// +1 octet for hop count

		if b := buff.Next(2); len(b) == 2 {
			if e := n.DestinationNet.UnmarshalBinary(b); e != nil {
				return e
			}
		} else {
			return errSliceTooShort
		}

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			destLen := int(b)
			n.DestinationLength = b

			if destLen > 0 {
				if b := buff.Next(destLen); len(b) == destLen {
					destMac := make(net.HardwareAddr, destLen)

					for i := 0; i < destLen; i++ {
						destMac[i] = b[i]
					}

					n.DestinationMAC = &destMac
				}
			}
		}

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			n.HopCount = b
		}
	}

	if hasSrc := n.ControlOctet & types.BIT3; hasSrc != 0 {
		// SNET, SLEN, SADR are present
		// Let's shave the bytes off
		// SNET = 2 octets
		// SLEN = 1 octet
		// SADR = SLEN
		if b := buff.Next(2); len(b) == 2 {
			if e := n.SourceNet.UnmarshalBinary(b); e != nil {
				return e
			}
		} else {
			return errSliceTooShort
		}

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			srcLen := int(b)
			n.SourceLength = b

			if srcLen > 0 {
				if b := buff.Next(srcLen); len(b) == srcLen {
					srcMac := make(net.HardwareAddr, srcLen)

					for i := 0; i < srcLen; i++ {
						srcMac[i] = b[i]
					}

					n.SourceMAC = &srcMac

					//n.SourceIP = []net.IP{n.SourceMAC[0], n.SourceMAC[1], n.SourceMAC[2], n.SourceMAC[3]}
				}
			}
		}
	}

	// expecting reply is third bit
	n.ExpectingReply = n.ControlOctet&types.BIT2 != 0

	// priority is first 2 bits
	n.Priority = types.MessagePriority(n.ControlOctet & types.BITS01)

	n.Length = len(b) - buff.Len()
	return nil
}
