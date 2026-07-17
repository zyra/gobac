package bacnet

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// TestSubscribeCovWithProcessIDLegacyBytesUnchanged pins the exact wire
// bytes produced by the legacy SubscribeCovWithProcessID delegation path
// (opts = {Confirmed: true, Indefinite: true}) to what the hardcoded
// pre-CovOptions marshal code produced, captured from the unmodified
// implementation before this change:
//
//	noncancel: 09 12 1c 00 80 00 07 29 01 39 00
//	cancel:    09 12 1c 00 80 00 07
func TestSubscribeCovWithProcessIDLegacyBytesUnchanged(t *testing.T) {
	object := types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 7}

	payload := newSubscribeCovPayload(object, CovOptions{
		ProcessID:  18,
		Confirmed:  true,
		Indefinite: true,
		Cancel:     false,
	})
	encoded, err := payload.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	wantNonCancel := []byte{0x09, 0x12, 0x1c, 0x00, 0x80, 0x00, 0x07, 0x29, 0x01, 0x39, 0x00}
	if !bytes.Equal(encoded, wantNonCancel) {
		t.Fatalf("legacy non-cancel SubscribeCOV = %x, want %x", encoded, wantNonCancel)
	}

	cancelPayload := newSubscribeCovPayload(object, CovOptions{
		ProcessID:  18,
		Confirmed:  true,
		Indefinite: true,
		Cancel:     true,
	})
	cancelEncoded, err := cancelPayload.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	wantCancel := []byte{0x09, 0x12, 0x1c, 0x00, 0x80, 0x00, 0x07}
	if !bytes.Equal(cancelEncoded, wantCancel) {
		t.Fatalf("legacy cancel SubscribeCOV = %x, want %x", cancelEncoded, wantCancel)
	}
}

// TestSubscribeCovWithOptionsUnconfirmedLifetime confirms the new options
// entry point produces the normative unconfirmed-with-lifetime wire form.
func TestSubscribeCovWithOptionsUnconfirmedLifetime(t *testing.T) {
	object := types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 7}
	payload := newSubscribeCovPayload(object, CovOptions{
		ProcessID:       18,
		Confirmed:       false,
		LifetimeSeconds: 300,
	})
	encoded, err := payload.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x09, 0x12, 0x1c, 0x00, 0x80, 0x00, 0x07, 0x29, 0x00, 0x3a, 0x01, 0x2c}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("options SubscribeCOV = %x, want %x", encoded, want)
	}
}

// TestUnconfirmedCovNotificationReachesCovHandler exercises the routing
// change in Server.handle: an UnconfirmedCOVNotification for a registered
// (deviceIP, processID) must reach the same covHandlers channel that
// confirmed notifications use, without opening real sockets.
func TestUnconfirmedCovNotificationReachesCovHandler(t *testing.T) {
	server := newLifecycleTestServer()
	deviceIP := net.IPv4(192, 0, 2, 77)
	sender := &net.UDPAddr{IP: deviceIP, Port: 47808}

	handler := make(chan *Request, 1)
	server.SetCovHandlerWithProcessID(deviceIP, 18, handler)
	defer server.RemoveCovHandlerWithProcessID(deviceIP, 18)

	// UnconfirmedCOVNotification-Request payload: process id 18, device
	// object (device,1), monitored object (analog-value,7)... reusing the
	// list-of-values tail from the existing CovNotification codec fixture.
	notificationPayload := []byte{
		0x09, 0x12, // [0] process identifier = 18
		0x1c, 0x02, 0x00, 0x00, 0x01, // [1] device object id (device,1)
		0x2c, 0x00, 0x80, 0x00, 0x07, // [2] monitored object id (analog-value,7)
		0x39, 0x00, // [3] time remaining = 0
		0x4e, 0x09, 0x55, 0x2e, 0x21, 0x01, 0x2f, 0x4f, // [4] list-of-values (opening/value/closing)
	}

	notify := NewRequest()
	notify.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	notify.Apdu.PduType = types.PduTypeUnconfirmedServiceRequest
	notify.Apdu.ServiceChoice = types.UnconfirmedServiceCovNotification
	notify.Apdu.Payload = notificationPayload
	raw, err := notify.MarshalBinary()
	notify.Release()
	if err != nil {
		t.Fatal(err)
	}

	server.handle(append([]byte(nil), raw...), len(raw), sender)

	select {
	case delivered := <-handler:
		defer delivered.Release()
		if delivered.ServiceChoice() != uint8(types.UnconfirmedServiceCovNotification) {
			t.Fatalf("delivered service choice = %d", delivered.ServiceChoice())
		}
	case <-time.After(time.Second):
		t.Fatal("unconfirmed COV notification was not delivered to the registered handler")
	}
}
