package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestReadPropertyMultipleRequestMarshal(t *testing.T) {
	request := &ReadPropertyMultiplePdu{
		Specs: []ReadAccessSpec{
			{
				ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 1},
				Properties: []PropertyReference{
					{ID: 85},
					{ID: 87, Index: 3, HasIndex: true},
				},
			},
		},
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x01, // [0] objectIdentifier AI:1
		0x1e,       // [1] opening
		0x09, 0x55, //   [0] propertyIdentifier 85
		0x09, 0x57, 0x19, 0x03, //   [0] propertyIdentifier 87, [1] propertyArrayIndex 3
		0x1f, // [1] closing
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded request = % x, want % x", encoded, want)
	}
}

func TestReadPropertyMultipleAckRoundTrip(t *testing.T) {
	ack := &ReadPropertyMultipleAck{
		Results: []ReadAccessResult{
			{
				ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 1},
				Results: []PropertyResult{
					{
						ID: 85,
						Values: []*types.PropertyValue{
							{Type: types.TagReal, Value: float32(22.5)},
						},
					},
					{
						ID: 87,
						Error: &PropertyAccessError{
							Class: types.ErrorClassProperty,
							Code:  types.ErrorCodeUnknownProperty,
						},
					},
				},
			},
		},
	}

	encoded, err := ack.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x01, // [0] objectIdentifier AI:1
		0x1e, // [1] opening (listOfResults)

		0x29, 0x55, //   [2] propertyIdentifier 85
		0x4e,                         //   [4] opening
		0x44, 0x41, 0xb4, 0x00, 0x00, //     Real 22.5
		0x4f, //   [4] closing

		0x29, 0x57, //   [2] propertyIdentifier 87
		0x5e,       //   [5] opening
		0x91, 0x02, //     errorClass Property(2)
		0x91, 0x20, //     errorCode UnknownProperty(32)
		0x5f, //   [5] closing

		0x1f, // [1] closing
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded ack = % x, want % x", encoded, want)
	}

	var decoded ReadPropertyMultipleAck
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}

	wantDecoded := ReadPropertyMultipleAck{
		Results: []ReadAccessResult{
			{
				ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 1},
				Results: []PropertyResult{
					{
						ID: 85,
						Values: []*types.PropertyValue{
							{Type: types.TagReal, Value: types.Real(22.5)},
						},
					},
					{
						ID: 87,
						Error: &PropertyAccessError{
							Class: types.ErrorClassProperty,
							Code:  types.ErrorCodeUnknownProperty,
						},
					},
				},
			},
		},
	}

	if len(decoded.Results) != len(wantDecoded.Results) {
		t.Fatalf("decoded %d results, want %d", len(decoded.Results), len(wantDecoded.Results))
	}
	gotResult := decoded.Results[0]
	wantResult := wantDecoded.Results[0]
	if *gotResult.ObjectId != *wantResult.ObjectId {
		t.Fatalf("decoded objectId = %+v, want %+v", *gotResult.ObjectId, *wantResult.ObjectId)
	}
	if len(gotResult.Results) != 2 {
		t.Fatalf("decoded %d property results, want 2", len(gotResult.Results))
	}

	gotValue := gotResult.Results[0]
	wantValue := wantResult.Results[0]
	if gotValue.ID != wantValue.ID || gotValue.HasIndex != wantValue.HasIndex || gotValue.Error != nil {
		t.Fatalf("decoded property[0] = %+v, want %+v", gotValue, wantValue)
	}
	if len(gotValue.Values) != 1 || *gotValue.Values[0] != *wantValue.Values[0] {
		t.Fatalf("decoded property[0] values = %+v, want %+v", gotValue.Values, wantValue.Values)
	}

	gotError := gotResult.Results[1]
	wantError := wantResult.Results[1]
	if gotError.ID != wantError.ID || gotError.Values != nil {
		t.Fatalf("decoded property[1] = %+v, want %+v", gotError, wantError)
	}
	if gotError.Error == nil || *gotError.Error != *wantError.Error {
		t.Fatalf("decoded property[1] error = %+v, want %+v", gotError.Error, wantError.Error)
	}
}

func TestReadPropertyMultipleAckDecodeSimulatorFormat(t *testing.T) {
	// Hand-built fixture following the exact byte layout produced by
	// simulator.encodeReadPropertyMultipleResult (simulator/services.go),
	// exercising an array index and a multi-value property result in
	// addition to the single-value/error cases covered above.
	wire := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x07, // [0] objectIdentifier AI:7
		0x1e, // [1] opening (listOfResults)

		0x29, 0x55, //   [2] propertyIdentifier 85
		0x39, 0x02, //   [3] propertyArrayIndex 2
		0x4e,                         //   [4] opening
		0x44, 0x40, 0x60, 0x00, 0x00, //     Real 3.5
		0x11, //     Boolean true
		0x4f, //   [4] closing

		0x29, 0x57, //   [2] propertyIdentifier 87
		0x5e,       //   [5] opening
		0x91, 0x01, //     errorClass Object(1)
		0x91, 0x1f, //     errorCode UnknownObject(31)
		0x5f, //   [5] closing

		0x1f, // [1] closing
	}

	var decoded ReadPropertyMultipleAck
	if err := decoded.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Results) != 1 {
		t.Fatalf("decoded %d results, want 1", len(decoded.Results))
	}
	result := decoded.Results[0]
	wantObjectID := types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 7}
	if *result.ObjectId != wantObjectID {
		t.Fatalf("decoded objectId = %+v, want %+v", *result.ObjectId, wantObjectID)
	}
	if len(result.Results) != 2 {
		t.Fatalf("decoded %d property results, want 2", len(result.Results))
	}

	value := result.Results[0]
	if value.ID != 85 || !value.HasIndex || value.Index != 2 || value.Error != nil {
		t.Fatalf("decoded property[0] = %+v", value)
	}
	if len(value.Values) != 2 {
		t.Fatalf("decoded property[0] has %d values, want 2", len(value.Values))
	}
	if value.Values[0].Type != types.TagReal || value.Values[0].Value != types.Real(3.5) {
		t.Fatalf("decoded property[0] value[0] = %+v", value.Values[0])
	}
	if value.Values[1].Type != types.TagBoolean || value.Values[1].Value != true {
		t.Fatalf("decoded property[0] value[1] = %+v", value.Values[1])
	}

	propError := result.Results[1]
	if propError.ID != 87 || propError.HasIndex || propError.Values != nil {
		t.Fatalf("decoded property[1] = %+v", propError)
	}
	wantError := PropertyAccessError{Class: types.ErrorClassObject, Code: types.ErrorCodeUnknownObject}
	if propError.Error == nil || *propError.Error != wantError {
		t.Fatalf("decoded property[1] error = %+v, want %+v", propError.Error, wantError)
	}
}
