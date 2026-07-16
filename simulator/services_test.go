package simulator

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/bacnet/types"
)

func TestReadPropertyServiceCodec(t *testing.T) {
	object := ObjectID{Type: uint16(types.ObjectTypeAnalogInput), Instance: 70000}
	reference := PropertyReference{ID: uint32(types.PropertyPresentValue)}
	objectID, err := toBACnetObjectID(object)
	if err != nil {
		t.Fatal(err)
	}
	request, err := (&types.Property{ObjectId: objectID, ID: types.PropertyPresentValue}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	gotObject, gotReference, err := decodeReadProperty(request)
	if err != nil {
		t.Fatal(err)
	}
	if gotObject != object || gotReference.ID != reference.ID || gotReference.ArrayIndex != nil {
		t.Fatalf("decoded object/reference = %+v/%+v", gotObject, gotReference)
	}

	ack, err := encodeReadPropertyResult(object, reference, []Value{{Tag: types.TagReal, Value: float32(21.5)}})
	if err != nil {
		t.Fatal(err)
	}
	var property types.Property
	if err := property.UnmarshalBinary(ack); err != nil {
		t.Fatal(err)
	}
	if property.ObjectId.InstanceNumber() != object.Instance || len(property.Values) != 1 || property.Values[0].ReadAsFloat64() != 21.5 {
		t.Fatalf("decoded acknowledgment = %+v", property)
	}
}

func TestWritePropertyServiceCodec(t *testing.T) {
	object := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 2}
	payload, err := encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagReal, Value: float32(42)}})
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, 0x49, 0x08)
	decodedObject, reference, values, priority, err := decodeWriteProperty(payload)
	if err != nil {
		t.Fatal(err)
	}
	if decodedObject != object || reference.ID != uint32(types.PropertyPresentValue) || priority != 8 || len(values) != 1 {
		t.Fatalf("decoded WriteProperty = %+v %+v %+v %d", decodedObject, reference, values, priority)
	}
}

func TestWhoIsAndIAmServiceCodecs(t *testing.T) {
	low, high, err := decodeWhoIs([]byte{0x09, 0x64, 0x1a, 0x01, 0x2c})
	if err != nil {
		t.Fatal(err)
	}
	if *low != 100 || *high != 300 {
		t.Fatalf("range = %d..%d", *low, *high)
	}
	if _, _, err := decodeWhoIs([]byte{0x09, 1}); err == nil {
		t.Fatal("accepted an unpaired Who-Is limit")
	}

	payload, err := encodeIAm(&Device{ID: 24111, VendorID: 260})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xc4, 0x02, 0x00, 0x5e, 0x2f, 0x22, 0x05, 0xc4, 0x91, 0x03, 0x22, 0x01, 0x04}
	if !bytes.Equal(payload, want) {
		t.Fatalf("I-Am payload = %x, want %x", payload, want)
	}
}

func TestReadPropertyMultipleServiceCodec(t *testing.T) {
	payload := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x21,
		0x1e,
		0x09, byte(types.PropertyPresentValue),
		0x09, byte(types.PropertyObjectName),
		0x19, 0,
		0x1f,
	}
	specifications, err := decodeReadPropertyMultiple(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(specifications) != 1 || len(specifications[0].Properties) != 2 || specifications[0].Properties[1].ArrayIndex == nil {
		t.Fatalf("decoded RPM request = %+v", specifications)
	}

	encoded, err := encodeReadPropertyMultipleResult([]ReadAccessResult{{
		Object: ObjectID{Type: 0, Instance: 33},
		Results: []PropertyResult{
			{Reference: PropertyReference{ID: uint32(types.PropertyPresentValue)}, Values: []Value{{Tag: types.TagReal, Value: float32(1.5)}}},
			{Reference: PropertyReference{ID: 999}, ErrorClass: types.ErrorClassProperty, ErrorCode: types.ErrorCodeUnknownProperty},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x21, 0x1e,
		0x29, byte(types.PropertyPresentValue), 0x4e, 0x44, 0x3f, 0xc0, 0x00, 0x00, 0x4f,
		0x2a, 0x03, 0xe7, 0x5e, 0x91, byte(types.ErrorClassProperty), 0x91, byte(types.ErrorCodeUnknownProperty), 0x5f,
		0x1f,
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("RPM result = %x, want %x", encoded, want)
	}
}

func TestSubscribeCOVServiceCodec(t *testing.T) {
	payload := []byte{
		0x0c, 0x01, 0x02, 0x03, 0x04,
		0x1c, 0x00, 0x00, 0x00, 0x21,
		0x29, 0x01,
		0x39, 60,
	}
	request, err := decodeSubscribeCOV(payload)
	if err != nil {
		t.Fatal(err)
	}
	if request.ProcessIdentifier != 0x01020304 || !request.Confirmed || request.Lifetime != 60 || request.Cancel {
		t.Fatalf("decoded SubscribeCOV = %+v", request)
	}
	cancel, err := decodeSubscribeCOV(payload[:10])
	if err != nil || !cancel.Cancel {
		t.Fatalf("decoded cancellation = %+v, %v", cancel, err)
	}
}

func TestSubscribeCOVAcceptsIndependentOptionalFields(t *testing.T) {
	prefix := []byte{0x09, 7, 0x1c, 0x00, 0x00, 0x00, 0x21}
	tests := []struct {
		name      string
		suffix    []byte
		confirmed bool
		lifetime  uint32
	}{
		{name: "confirmed only", suffix: []byte{0x29, 1}, confirmed: true},
		{name: "lifetime only", suffix: []byte{0x39, 60}, lifetime: 60},
	}
	for _, test := range tests {
		request, err := decodeSubscribeCOV(append(append([]byte(nil), prefix...), test.suffix...))
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if request.Cancel || request.Confirmed != test.confirmed || request.Lifetime != test.lifetime {
			t.Fatalf("%s decoded as %+v", test.name, request)
		}
	}
}
