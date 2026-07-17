package simulator

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
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

func TestWritePropertyPriorityOutOfRange(t *testing.T) {
	object := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 2}
	payload, err := encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagReal, Value: float32(42)}})
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, 0x49, 17)
	if _, _, _, _, err := decodeWriteProperty(payload); err != errWritePriorityOutOfRange {
		t.Fatalf("priority 17 error = %v", err)
	}
}

func TestWritePropertyMultipleServiceCodec(t *testing.T) {
	analogID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	binaryID := ObjectID{Type: uint16(types.ObjectTypeBinaryValue), Instance: 2}
	payload := []byte{
		0x0c, 0x00, 0x80, 0x00, 0x01, // [0] objectIdentifier AV:1
		0x1e,       // [1] opening
		0x09, 0x55, //   [0] propertyIdentifier 85
		0x2e,                         //   [2] opening
		0x44, 0x41, 0xa8, 0x00, 0x00, //     Real 21.0
		0x2f,       //   [2] closing
		0x39, 0x08, //   [3] priority 8
		0x1f, // [1] closing

		0x0c, 0x01, 0x40, 0x00, 0x02, // [0] objectIdentifier BV:2
		0x1e,       // [1] opening
		0x09, 0x55, //   [0] propertyIdentifier 85
		0x2e,       //   [2] opening
		0x91, 0x01, //     Enumerated 1
		0x2f, //   [2] closing
		0x1f, // [1] closing (no priority)
	}

	specifications, err := decodeWritePropertyMultiple(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(specifications) != 2 {
		t.Fatalf("decoded %d specifications, want 2", len(specifications))
	}

	first := specifications[0]
	if first.Object != analogID || len(first.Properties) != 1 {
		t.Fatalf("decoded first specification = %+v", first)
	}
	firstWrite := first.Properties[0]
	if firstWrite.Reference.ID != uint32(types.PropertyPresentValue) || firstWrite.Reference.ArrayIndex != nil || firstWrite.Priority != 8 {
		t.Fatalf("decoded first write = %+v", firstWrite)
	}
	if len(firstWrite.Values) != 1 || firstWrite.Values[0].Tag != types.TagReal || firstWrite.Values[0].Value != types.Real(21.0) {
		t.Fatalf("decoded first write values = %+v", firstWrite.Values)
	}

	second := specifications[1]
	if second.Object != binaryID || len(second.Properties) != 1 {
		t.Fatalf("decoded second specification = %+v", second)
	}
	secondWrite := second.Properties[0]
	if secondWrite.Reference.ID != uint32(types.PropertyPresentValue) || secondWrite.Reference.ArrayIndex != nil || secondWrite.Priority != 0 {
		t.Fatalf("decoded second write = %+v", secondWrite)
	}
	if len(secondWrite.Values) != 1 || secondWrite.Values[0].Tag != types.TagEnumerated || secondWrite.Values[0].Value != uint32(1) {
		t.Fatalf("decoded second write values = %+v", secondWrite.Values)
	}
}

func TestWritePropertyMultiplePriorityOutOfRange(t *testing.T) {
	payload := []byte{
		0x0c, 0x00, 0x80, 0x00, 0x01, // [0] objectIdentifier AV:1
		0x1e,       // [1] opening
		0x09, 0x55, //   [0] propertyIdentifier 85
		0x2e,                         //   [2] opening
		0x44, 0x41, 0xa8, 0x00, 0x00, //     Real 21.0
		0x2f,     //   [2] closing
		0x39, 17, //   [3] priority 17 (out of range)
		0x1f, // [1] closing
	}
	if _, err := decodeWritePropertyMultiple(payload); err != errWritePriorityOutOfRange {
		t.Fatalf("priority 17 error = %v", err)
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
