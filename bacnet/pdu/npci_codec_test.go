package pdu

import "testing"

func TestNpciRejectsTruncatedAddresses(t *testing.T) {
	packet := []byte{
		0x01, 0x28,
		0x12, 0x34, 0x02, 0xaa, 0xbb, 0xff,
		0x56, 0x78, 0x02, 0xcc, 0xdd,
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
