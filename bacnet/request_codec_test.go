package bacnet

import (
	"net"
	"testing"

	"github.com/zyra/gobac/bacnet/types"
)

func TestParseRequestRequiresExactBVLCLength(t *testing.T) {
	request := NewRequest()
	request.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	request.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	request.Apdu.ServiceChoice = types.UnconfirmedServiceWhoIs
	encoded, err := request.MarshalBinary()
	request.Release()
	if err != nil {
		t.Fatal(err)
	}
	sender := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 47808}
	for _, malformed := range [][]byte{
		append(append([]byte(nil), encoded...), 0),
		{0x81, 0x0a, 0x00, 0x03, 0x01, 0x00},
	} {
		if parsed, err := ParseRequest(malformed, sender); err == nil {
			parsed.Release()
			t.Fatalf("accepted malformed BVLC frame %x", malformed)
		}
	}
}
