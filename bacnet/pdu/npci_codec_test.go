package pdu

import (
	"bytes"
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestNpciMarshalLocalMessagesUseTwoOctets(t *testing.T) {
	for _, expectingReply := range []bool{false, true} {
		npci := Npci{ProtocolVersion: 1, ExpectingReply: expectingReply}
		encoded, err := npci.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		control := byte(0)
		if expectingReply {
			control = types.BIT2
		}
		if want := []byte{1, control}; !bytes.Equal(encoded, want) {
			t.Fatalf("expecting reply %v encoded %x, want %x", expectingReply, encoded, want)
		}
	}
}

func TestNpciMarshalRoutedAddresses(t *testing.T) {
	destination := net.HardwareAddr{0xaa, 0xbb}
	source := net.HardwareAddr{0xcc}
	npci := Npci{
		ProtocolVersion: 1,
		ExpectingReply:  true,
		DestinationNet:  0x1234,
		DestinationMAC:  &destination,
		SourceNet:       0x5678,
		SourceMAC:       &source,
		HopCount:        0xff,
	}
	encoded, err := npci.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 0x2c, 0x12, 0x34, 2, 0xaa, 0xbb, 0x56, 0x78, 1, 0xcc, 0xff}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded %x, want %x", encoded, want)
	}

	var decoded Npci
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Length != len(encoded) || decoded.DestinationNet != npci.DestinationNet || decoded.SourceNet != npci.SourceNet {
		t.Fatalf("unexpected decoded NPCI: %+v", decoded)
	}
}

func TestNpciCanDecodeLocalAfterRouted(t *testing.T) {
	var npci Npci
	routed := []byte{1, 0x28, 0x12, 0x34, 1, 0xaa, 0x56, 0x78, 1, 0xcc, 0xff}
	if err := npci.UnmarshalBinary(routed); err != nil {
		t.Fatal(err)
	}
	if err := npci.UnmarshalBinary([]byte{1, 0}); err != nil {
		t.Fatal(err)
	}
	if npci.Length != 2 || npci.DestinationNet != 0 || npci.SourceNet != 0 || npci.DestinationMAC != nil || npci.SourceMAC != nil {
		t.Fatalf("local NPCI retained routed state: %+v", npci)
	}
}

func TestNpciRejectsTruncatedAddresses(t *testing.T) {
	packet := []byte{
		0x01, 0x28,
		0x12, 0x34, 0x02, 0xaa, 0xbb,
		0x56, 0x78, 0x02, 0xcc, 0xdd, 0xff,
	}
	for length := 0; length < len(packet); length++ {
		var npci Npci
		if err := npci.UnmarshalBinary(packet[:length]); err == nil {
			t.Fatalf("accepted NPCI truncated to %d octets", length)
		}
	}
	var npci Npci
	if err := npci.UnmarshalBinary(packet); err != nil {
		t.Fatal(err)
	}
	if npci.Length != len(packet) {
		t.Fatalf("decoded length = %d, want %d", npci.Length, len(packet))
	}
}
