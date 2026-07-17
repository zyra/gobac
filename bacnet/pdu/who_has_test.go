package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestWhoHasByObjectIdRoundTrip(t *testing.T) {
	request := &WhoHas{
		ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3},
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0x2c, 0x00, 0x00, 0x00, 0x03}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded who-has by id = % x, want % x", encoded, want)
	}

	decoded := &WhoHas{}
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.HasRange {
		t.Fatalf("decoded.HasRange = true, want false")
	}
	if decoded.ObjectId == nil || *decoded.ObjectId != *request.ObjectId {
		t.Fatalf("decoded.ObjectId = %+v, want %+v", decoded.ObjectId, request.ObjectId)
	}
	if decoded.ObjectName != "" {
		t.Fatalf("decoded.ObjectName = %q, want empty", decoded.ObjectName)
	}
}

func TestWhoHasByNameWithRangeRoundTrip(t *testing.T) {
	request := &WhoHas{
		HasRange:   true,
		LowLimit:   1,
		HighLimit:  99,
		ObjectName: "pump",
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0x09, 0x01, 0x19, 0x63, 0x3d, 0x05, 0x00, 0x70, 0x75, 0x6d, 0x70}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded who-has by name = % x, want % x", encoded, want)
	}

	decoded := &WhoHas{}
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.HasRange || decoded.LowLimit != 1 || decoded.HighLimit != 99 {
		t.Fatalf("decoded range = HasRange:%v Low:%d High:%d", decoded.HasRange, decoded.LowLimit, decoded.HighLimit)
	}
	if decoded.ObjectId != nil {
		t.Fatalf("decoded.ObjectId = %+v, want nil", decoded.ObjectId)
	}
	if decoded.ObjectName != "pump" {
		t.Fatalf("decoded.ObjectName = %q, want %q", decoded.ObjectName, "pump")
	}
}

func TestWhoHasMarshalRequiresExactlyOneSelector(t *testing.T) {
	both := &WhoHas{ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3}, ObjectName: "pump"}
	if _, err := both.MarshalBinary(); err == nil {
		t.Fatal("expected an error when both ObjectId and ObjectName are set")
	}

	neither := &WhoHas{}
	if _, err := neither.MarshalBinary(); err == nil {
		t.Fatal("expected an error when neither ObjectId nor ObjectName is set")
	}
}

func TestIHaveRoundTrip(t *testing.T) {
	request := &IHave{
		DeviceId:   types.ObjectId{Type: types.ObjectTypeDevice, Instance: 1234},
		ObjectId:   types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3},
		ObjectName: "pump",
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0xc4, 0x02, 0x00, 0x04, 0xd2, // deviceIdentifier Device:1234
		0xc4, 0x00, 0x00, 0x00, 0x03, // objectIdentifier AI:3
		0x75, 0x05, 0x00, 0x70, 0x75, 0x6d, 0x70, // objectName "pump"
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded i-have = % x, want % x", encoded, want)
	}

	decoded := &IHave{}
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DeviceId != request.DeviceId {
		t.Fatalf("decoded.DeviceId = %+v, want %+v", decoded.DeviceId, request.DeviceId)
	}
	if decoded.ObjectId != request.ObjectId {
		t.Fatalf("decoded.ObjectId = %+v, want %+v", decoded.ObjectId, request.ObjectId)
	}
	if decoded.ObjectName != "pump" {
		t.Fatalf("decoded.ObjectName = %q, want %q", decoded.ObjectName, "pump")
	}
}
