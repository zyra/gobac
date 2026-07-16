package pdu

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/zyra/gobac/v2/bacnet/types"
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
	DestinationIP     net.IP

	SourceNet    types.Uint16
	SourceLength uint8
	SourceMAC    *net.HardwareAddr
	SourceIP     net.IP
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

	destination := []byte(nil)
	if n.DestinationMAC != nil {
		destination = []byte(*n.DestinationMAC)
	}
	if len(destination) > types.MaxMacLen {
		return nil, errors.New("destination MAC is too long")
	}
	if n.DestinationLength != 0 && int(n.DestinationLength) != len(destination) {
		return nil, errors.New("destination MAC length does not match address")
	}

	source := []byte(nil)
	if n.SourceMAC != nil {
		source = []byte(*n.SourceMAC)
	}
	if len(source) > types.MaxMacLen {
		return nil, errors.New("source MAC is too long")
	}
	if n.SourceLength != 0 && int(n.SourceLength) != len(source) {
		return nil, errors.New("source MAC length does not match address")
	}
	if n.SourceNet != 0 && len(source) == 0 {
		return nil, errors.New("source network requires a source MAC")
	}

	length := 2
	if n.DestinationNet != 0 {
		length += 2 + 1 + len(destination) + 1
	}
	if n.SourceNet != 0 {
		length += 2 + 1 + len(source)
	}
	b = make([]byte, length)
	b[0] = byte(n.ProtocolVersion)
	offset := 2

	if n.NetworkLayerMessage {
		b[1] |= types.BIT7
	}

	if n.DestinationNet > 0 {
		b[1] |= types.BIT5
		if v, e := n.DestinationNet.MarshalBinary(); e != nil {
			return nil, e
		} else {
			copy(b[offset:], v)
		}
		offset += 2
		b[offset] = byte(len(destination))
		offset++
		copy(b[offset:], destination)
		offset += len(destination)
	}

	if n.SourceNet > 0 {
		b[1] |= types.BIT3
		if v, e := n.SourceNet.MarshalBinary(); e != nil {
			return nil, e
		} else {
			copy(b[offset:], v)
		}
		offset += 2
		b[offset] = byte(len(source))
		offset++
		copy(b[offset:], source)
		offset += len(source)
	}

	if n.DestinationNet > 0 {
		b[offset] = byte(n.HopCount)
	}

	if n.ExpectingReply {
		b[1] |= types.BIT2
	}

	b[1] |= byte(n.Priority) & types.BITS01

	return b, e
}

func (n *Npci) UnmarshalBinary(b []byte) error {
	n.Reset()
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
				address := buff.Next(destLen)
				if len(address) != destLen {
					return errSliceTooShort
				}
				destMac := append(net.HardwareAddr(nil), address...)
				n.DestinationMAC = &destMac
			}
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
				address := buff.Next(srcLen)
				if len(address) != srcLen {
					return errSliceTooShort
				}
				srcMac := append(net.HardwareAddr(nil), address...)
				n.SourceMAC = &srcMac
			}
		}
	}

	if n.DestinationNet != 0 {
		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			n.HopCount = b
		}
	}

	// expecting reply is third bit
	n.ExpectingReply = n.ControlOctet&types.BIT2 != 0

	// priority is first 2 bits
	n.Priority = types.MessagePriority(n.ControlOctet & types.BITS01)

	n.Length = len(b) - buff.Len()
	return nil
}
