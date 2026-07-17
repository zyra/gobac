package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestWritePropertyMultipleRequestRoundTrip(t *testing.T) {
	request := &WritePropertyMultiplePdu{
		Specs: []WriteAccessSpec{
			{
				ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1},
				Properties: []PropertyWrite{
					{
						ID:          85,
						Values:      []*types.PropertyValue{{Type: types.TagReal, Value: float32(21.0)}},
						Priority:    8,
						HasPriority: true,
					},
				},
			},
			{
				ObjectId: &types.ObjectId{Type: types.ObjectTypeBinaryValue, Instance: 2},
				Properties: []PropertyWrite{
					{
						ID:     85,
						Values: []*types.PropertyValue{{Type: types.TagEnumerated, Value: uint32(1)}},
					},
				},
			},
		},
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
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
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded request = % x, want % x", encoded, want)
	}

	var decoded WritePropertyMultiplePdu
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Specs) != 2 {
		t.Fatalf("decoded %d specs, want 2", len(decoded.Specs))
	}

	first := decoded.Specs[0]
	wantFirstObject := types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1}
	if *first.ObjectId != wantFirstObject {
		t.Fatalf("decoded first objectId = %+v, want %+v", *first.ObjectId, wantFirstObject)
	}
	if len(first.Properties) != 1 {
		t.Fatalf("decoded %d properties for first spec, want 1", len(first.Properties))
	}
	firstProperty := first.Properties[0]
	if firstProperty.ID != 85 || firstProperty.HasIndex || !firstProperty.HasPriority || firstProperty.Priority != 8 {
		t.Fatalf("decoded first property = %+v", firstProperty)
	}
	if len(firstProperty.Values) != 1 || firstProperty.Values[0].Type != types.TagReal || firstProperty.Values[0].Value != types.Real(21.0) {
		t.Fatalf("decoded first property values = %+v", firstProperty.Values)
	}

	second := decoded.Specs[1]
	wantSecondObject := types.ObjectId{Type: types.ObjectTypeBinaryValue, Instance: 2}
	if *second.ObjectId != wantSecondObject {
		t.Fatalf("decoded second objectId = %+v, want %+v", *second.ObjectId, wantSecondObject)
	}
	if len(second.Properties) != 1 {
		t.Fatalf("decoded %d properties for second spec, want 1", len(second.Properties))
	}
	secondProperty := second.Properties[0]
	if secondProperty.ID != 85 || secondProperty.HasIndex || secondProperty.HasPriority {
		t.Fatalf("decoded second property = %+v", secondProperty)
	}
	if len(secondProperty.Values) != 1 || secondProperty.Values[0].Type != types.TagEnumerated || secondProperty.Values[0].Value != uint32(1) {
		t.Fatalf("decoded second property values = %+v", secondProperty.Values)
	}
}

func TestWritePropertyMultipleRequestValidation(t *testing.T) {
	if _, err := (&WritePropertyMultiplePdu{}).MarshalBinary(); err == nil {
		t.Fatal("expected an error for an empty request")
	}
	if _, err := (&WriteAccessSpec{}).MarshalBinary(); err == nil {
		t.Fatal("expected an error for a missing object identifier")
	}
	spec := &WriteAccessSpec{ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1}}
	if _, err := spec.MarshalBinary(); err == nil {
		t.Fatal("expected an error for a missing property list")
	}
}

func TestWritePropertyMultipleErrorRoundTrip(t *testing.T) {
	object := &types.ObjectId{Type: types.ObjectTypeBinaryValue, Instance: 2}
	wpmError := &WritePropertyMultipleError{
		Class: types.ErrorClassProperty,
		Code:  types.ErrorCodeWriteAccessDenied,
	}
	wpmError.FirstFailed.ObjectId = object
	wpmError.FirstFailed.ID = 85

	encoded, err := wpmError.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x0e,       // [0] opening (errorType)
		0x91, 0x02, //   errorClass Property(2)
		0x91, 0x28, //   errorCode WriteAccessDenied(40)
		0x0f, // [0] closing

		0x1e,                         // [1] opening (firstFailedWriteAttempt)
		0x0c, 0x01, 0x40, 0x00, 0x02, //   [0] objectIdentifier BV:2
		0x19, 0x55, //   [1] propertyIdentifier 85
		0x1f, // [1] closing
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded error = % x, want % x", encoded, want)
	}

	var decoded WritePropertyMultipleError
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Class != types.ErrorClassProperty || decoded.Code != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("decoded class/code = %v/%v", decoded.Class, decoded.Code)
	}
	if decoded.FirstFailed.ObjectId == nil || *decoded.FirstFailed.ObjectId != *object {
		t.Fatalf("decoded firstFailed object = %+v", decoded.FirstFailed.ObjectId)
	}
	if decoded.FirstFailed.ID != 85 || decoded.FirstFailed.HasIndex {
		t.Fatalf("decoded firstFailed property = %+v", decoded.FirstFailed)
	}
}
