package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/bacnet/types"
)

func TestSubscribeCovUsesCompactUnsignedLengths(t *testing.T) {
	objectID := &types.ObjectId{Type: types.ObjectTypeDevice}
	if err := objectID.SetInstanceNumber(123); err != nil {
		t.Fatal(err)
	}
	request := SubscribeCov{
		ProcessIdentifier32: 0x01020304,
		ObjectId:            objectID,
		Timeout:             60,
	}
	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0c, 0x01, 0x02, 0x03, 0x04,
		0x1c, 0x02, 0x00, 0x00, 0x7b,
		0x29, 0x01,
		0x39, 0x3c,
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded SubscribeCOV = %x, want %x", encoded, want)
	}
}

func TestSubscribeCovCancellationOmitsOptionalFields(t *testing.T) {
	objectID := &types.ObjectId{Type: types.ObjectTypeDevice, Instance: 1}
	request := SubscribeCov{ProcessIdentifier: 7, ObjectId: objectID, Cancel: true}
	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x09, 0x07, 0x1c, 0x02, 0x00, 0x00, 0x01}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded cancellation = %x, want %x", encoded, want)
	}
}

func TestCovNotificationAcceptsUnsigned32ProcessIdentifier(t *testing.T) {
	wire := []byte{
		0x0c, 0xff, 0xff, 0xff, 0xff,
		0x1c, 0x02, 0x00, 0x00, 0x01,
		0x2c, 0x00, 0x00, 0x00, 0x01,
		0x39, 0x00,
		0x4e, 0x4f,
	}
	var notification CovNotification
	if err := notification.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if notification.ProcessIdentifier32 != ^uint32(0) {
		t.Fatalf("process identifier = %d", notification.ProcessIdentifier32)
	}
}

func TestWritePropertyRejectsInvalidPriority(t *testing.T) {
	request := WriteProperty{
		Property: &types.Property{ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogOutput, Instance: 1}, ID: types.PropertyPresentValue, Values: []*types.PropertyValue{{Type: types.TagReal, Value: float32(1)}}},
		Priority: 17,
	}
	if _, err := request.MarshalBinary(); err == nil {
		t.Fatal("priority 17 was accepted")
	}
}
