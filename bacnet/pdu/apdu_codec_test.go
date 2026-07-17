package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestApduMarshalByPduType(t *testing.T) {
	tests := []struct {
		name string
		apdu Apdu
		want []byte
	}{
		{"unconfirmed request", Apdu{PduType: types.PduTypeUnconfirmedServiceRequest, ServiceChoice: types.UnconfirmedServiceWhoIs}, []byte{0x10, 0x08}},
		{"confirmed request with zero invoke ID", Apdu{PduType: types.PduTypeConfirmedServiceRequest, MaxApdu: 1476, InvokeID: 0, ServiceChoice: types.ConfirmedServiceReadProperty}, []byte{0x00, 0x05, 0x00, 0x0c}},
		{"simple ack", Apdu{PduType: types.PduTypeSimpleAck, InvokeID: 0x7e, ServiceChoice: types.ConfirmedServiceWriteProperty}, []byte{0x20, 0x7e, 0x0f}},
		{"error", Apdu{PduType: types.PduTypeError, InvokeID: 0x7e, ServiceChoice: types.ConfirmedServiceReadProperty, ErrorClass: 2, ErrorCode: 32}, []byte{0x50, 0x7e, 0x0c, 0x91, 0x02, 0x91, 0x20}},
		{"abort", Apdu{PduType: types.PduTypeAbort, InvokeID: 0x7e, AbortReason: types.AbortReasonOther}, []byte{0x70, 0x7e, 0x00}},
	}

	for _, test := range tests {
		encoded, err := test.apdu.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, test.want) {
			t.Errorf("%s encoded as %x, want %x", test.name, encoded, test.want)
		}
	}
}

func TestAbortPreservesInvokeID(t *testing.T) {
	var apdu Apdu
	if err := apdu.UnmarshalBinary([]byte{0x70, 0x9a, 0x00}); err != nil {
		t.Fatal(err)
	}
	if apdu.InvokeID != 0x9a {
		t.Fatalf("invoke ID = %x, want 9a", apdu.InvokeID)
	}
}

func TestApduPreservesLowBitMetadata(t *testing.T) {
	confirmed := Apdu{PduType: types.PduTypeConfirmedServiceRequest, MaxApdu: 1476, InvokeID: 1, ServiceChoice: 99, SegmentedResponseAccepted: true}
	encoded, err := confirmed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[0] != 0x02 {
		t.Fatalf("confirmed flags = %#x", encoded[0])
	}
	var decoded Apdu
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.SegmentedResponseAccepted {
		t.Fatal("segmented-response-accepted flag was lost")
	}

	abort := Apdu{PduType: types.PduTypeAbort, InvokeID: 2, AbortReason: types.AbortReasonOther, Server: true}
	encoded, err = abort.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[0] != 0x71 {
		t.Fatalf("abort flags = %#x", encoded[0])
	}
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Server {
		t.Fatal("abort server flag was lost")
	}
}

func TestSegmentedConfirmedRequestHeaderIsDecoded(t *testing.T) {
	wire := []byte{0x0c, 0x05, 37, 2, 4, byte(types.ConfirmedServiceReadProperty), 0xaa}
	var apdu Apdu
	if err := apdu.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if !apdu.Segmented || !apdu.MoreFollows || apdu.InvokeID != 37 || apdu.SequenceNumber != 2 || apdu.ProposedWindowSize != 4 || apdu.ServiceChoice != types.ConfirmedServiceReadProperty {
		t.Fatalf("decoded segmented request = %+v", apdu)
	}
	if !bytes.Equal(apdu.Payload, []byte{0xaa}) {
		t.Fatalf("payload = %x", apdu.Payload)
	}
}

func TestErrorApduDecoding(t *testing.T) {
	var apdu Apdu
	if err := apdu.UnmarshalBinary([]byte{0x50, 0x7e, 0x0c, 0x91, 0x02, 0x91, 0x20, 0xaa}); err != nil {
		t.Fatal(err)
	}
	if !apdu.Errored || apdu.InvokeID != 0x7e || apdu.ServiceChoice != 0x0c || apdu.ErrorClass != 2 || apdu.ErrorCode != 32 {
		t.Fatalf("decoded error APDU incorrectly: %+v", apdu)
	}
	if !bytes.Equal(apdu.Payload, []byte{0xaa}) {
		t.Fatalf("error payload = %x", apdu.Payload)
	}
}

func TestErrorApduDecodingWritePropertyMultiple(t *testing.T) {
	// Hand-built WritePropertyMultiple-Error production: errorClass Property
	// (2), errorCode WriteAccessDenied (40), firstFailedWriteAttempt =
	// BinaryValue:2 / property 85.
	wire := []byte{
		0x50, 0x7e, byte(types.ConfirmedServiceWritePropertyMultiple),
		0x0e,       // [0] opening (errorType)
		0x91, 0x02, //   errorClass Property(2)
		0x91, 0x28, //   errorCode WriteAccessDenied(40)
		0x0f, // [0] closing

		0x1e,                         // [1] opening (firstFailedWriteAttempt)
		0x0c, 0x01, 0x40, 0x00, 0x02, //   [0] objectIdentifier BV:2
		0x19, 0x55, //   [1] propertyIdentifier 85
		0x1f, // [1] closing
	}

	var apdu Apdu
	if err := apdu.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if !apdu.Errored || apdu.InvokeID != 0x7e || apdu.ServiceChoice != types.ConfirmedServiceWritePropertyMultiple {
		t.Fatalf("decoded error APDU incorrectly: %+v", apdu)
	}
	if apdu.ErrorClass != types.ErrorClassProperty || apdu.ErrorCode != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("decoded class/code = %v/%v", apdu.ErrorClass, apdu.ErrorCode)
	}

	wpmError, ok := apdu.ResponseData.(*WritePropertyMultipleError)
	if !ok {
		t.Fatalf("ResponseData = %T, want *WritePropertyMultipleError", apdu.ResponseData)
	}
	if wpmError.Class != types.ErrorClassProperty || wpmError.Code != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("wpmError class/code = %v/%v", wpmError.Class, wpmError.Code)
	}
	wantObject := types.ObjectId{Type: types.ObjectTypeBinaryValue, Instance: 2}
	if wpmError.FirstFailed.ObjectId == nil || *wpmError.FirstFailed.ObjectId != wantObject {
		t.Fatalf("wpmError firstFailed object = %+v, want %+v", wpmError.FirstFailed.ObjectId, wantObject)
	}
	if wpmError.FirstFailed.ID != 85 || wpmError.FirstFailed.HasIndex {
		t.Fatalf("wpmError firstFailed property = %+v", wpmError.FirstFailed)
	}
}

func TestUnconfirmedCovNotificationIsDecoded(t *testing.T) {
	payload := []byte{
		0x09, 1,
		0x1c, 0x02, 0x00, 0x00, 0x01,
		0x2c, 0x00, 0x00, 0x00, 0x01,
		0x39, 0,
		0x4e,
		0x09, 0x55, 0x2e, 0x21, 1, 0x2f, 0x39, 1,
		0x09, 0x51, 0x2e, 0x21, 2, 0x2f, 0x39, 16,
		0x4f,
	}
	var apdu Apdu
	if err := apdu.UnmarshalBinary(append([]byte{0x10, byte(types.UnconfirmedServiceCovNotification)}, payload...)); err != nil {
		t.Fatal(err)
	}
	notification, ok := apdu.ResponseData.(*CovNotification)
	if !ok || len(notification.Properties) != 2 {
		t.Fatalf("notification = %#v", apdu.ResponseData)
	}
	if notification.Properties[0].Priority != 1 || notification.Properties[1].Priority != 16 {
		t.Fatalf("property priorities = %d, %d", notification.Properties[0].Priority, notification.Properties[1].Priority)
	}
}

func TestApduCopiesRawServicePayload(t *testing.T) {
	wire := []byte{0x00, 0x05, 0x2a, 0xff, 1, 2, 3}
	var apdu Apdu
	if err := apdu.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if want := []byte{1, 2, 3}; !bytes.Equal(apdu.Payload, want) {
		t.Fatalf("payload = %x, want %x", apdu.Payload, want)
	}
	wire[4] = 9
	if apdu.Payload[0] != 1 {
		t.Fatal("payload aliases the receive buffer")
	}
}

func TestApduRejectsTruncatedInput(t *testing.T) {
	inputs := [][]byte{
		nil,
		{0x00},
		{0x00, 0x05},
		{0x00, 0x05, 0x01},
		{0x10},
		{0x20},
		{0x20, 0x01},
		{0x30},
		{0x30, 0x01},
		{0x50, 0x01, 0x0c, 0x91},
		{0x60, 0x01},
		{0x70, 0x01},
	}

	for _, input := range inputs {
		var apdu Apdu
		if err := apdu.UnmarshalBinary(input); err == nil {
			t.Errorf("UnmarshalBinary(%x) succeeded", input)
		}
	}
}

func TestHistoricalReadPropertyFixture(t *testing.T) {
	fixture := []byte{
		0x30, 0x01, 0x0c, 0x0c, 0x02, 0x00, 0x5e, 0x2f, 0x1a, 0x01, 0x73, 0x3e,
		0x91, 0x70, 0x91, 0x79, 0x91, 0x78, 0x91, 0x46, 0x91, 0x2c, 0x91, 0x0c,
		0x91, 0x62, 0x91, 0x8b, 0x91, 0x61, 0x91, 0x60, 0x91, 0x4c, 0x91, 0x3e,
		0x91, 0x6b, 0x91, 0x0b, 0x91, 0x49, 0x91, 0x1e, 0x91, 0x9b, 0x91, 0x1c,
		0x91, 0x39, 0x91, 0x77, 0x91, 0x38, 0x91, 0x18, 0x91, 0x3a, 0x91, 0x98,
		0x91, 0x74, 0x91, 0xcc, 0x91, 0xc1, 0x91, 0xc3, 0x3f,
	}
	var apdu Apdu
	for length := 0; length < len(fixture); length++ {
		var truncated Apdu
		if err := truncated.UnmarshalBinary(fixture[:length]); err == nil {
			t.Fatalf("accepted ReadProperty fixture truncated to %d octets", length)
		}
	}
	if err := apdu.UnmarshalBinary(fixture); err != nil {
		t.Fatal(err)
	}
	response, ok := apdu.ResponseData.(*ReadPropertyPdu)
	if !ok {
		t.Fatalf("response type = %T", apdu.ResponseData)
	}
	if len(response.Property.Values) != 28 {
		t.Fatalf("decoded %d values, want 28", len(response.Property.Values))
	}
}

func TestHistoricalIAmFixtureAndTruncations(t *testing.T) {
	fixture := []byte{
		0x10, 0x00, 0xc4, 0x02, 0x00, 0x5e, 0x2f, 0x22, 0x05, 0xc4, 0x91, 0x03,
		0x22, 0x01, 0x04,
	}
	for length := 0; length < len(fixture); length++ {
		var truncated Apdu
		if err := truncated.UnmarshalBinary(fixture[:length]); err == nil {
			t.Fatalf("accepted I-Am fixture truncated to %d octets", length)
		}
	}
	var apdu Apdu
	if err := apdu.UnmarshalBinary(fixture); err != nil {
		t.Fatal(err)
	}
	device, ok := apdu.ResponseData.(*types.Device)
	if !ok {
		t.Fatalf("response type = %T", apdu.ResponseData)
	}
	if device.ObjectId.InstanceNumber() != 24111 {
		t.Fatalf("device instance = %d, want 24111", device.ObjectId.InstanceNumber())
	}
}

func TestHistoricalCovFixtureAndTruncations(t *testing.T) {
	fixture := []byte{
		0x00, 0x05, 0x40, 0x01, 0x09, 0x02, 0x1c, 0x02, 0x00, 0x15, 0x64, 0x2c,
		0x04, 0xc0, 0x00, 0x01, 0x3b, 0x28, 0xf9, 0x44, 0x4e, 0x09, 0x55, 0x2e,
		0x91, 0x02, 0x2f, 0x09, 0x6f, 0x2e, 0x82, 0x04, 0x10, 0x2f, 0x4f,
	}
	for length := 0; length < len(fixture); length++ {
		var truncated Apdu
		if err := truncated.UnmarshalBinary(fixture[:length]); err == nil {
			t.Fatalf("accepted COV fixture truncated to %d octets", length)
		}
	}
	var apdu Apdu
	if err := apdu.UnmarshalBinary(fixture); err != nil {
		t.Fatal(err)
	}
	notification, ok := apdu.ResponseData.(*CovNotification)
	if !ok {
		t.Fatalf("response type = %T", apdu.ResponseData)
	}
	if len(notification.Properties) != 2 {
		t.Fatalf("decoded %d properties, want 2", len(notification.Properties))
	}
}

func TestMalformedPduDecoderCorpus(t *testing.T) {
	state := uint32(0x9e3779b9)
	for size := 0; size < 64; size++ {
		data := make([]byte, size)
		for i := range data {
			state = state*1103515245 + 12345
			data[i] = byte(state >> 16)
		}

		var apdu Apdu
		_ = apdu.UnmarshalBinary(data)
		var npci Npci
		_ = npci.UnmarshalBinary(data)
		var cov CovNotification
		_ = cov.UnmarshalBinary(data)
	}
}
