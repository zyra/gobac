package types

import (
	"bytes"
	"math"
	"testing"
)

func TestSignedIntegerEncoding(t *testing.T) {
	tests := []struct {
		value int32
		want  []byte
	}{
		{0, []byte{0x00}},
		{127, []byte{0x7f}},
		{128, []byte{0x00, 0x80}},
		{-1, []byte{0xff}},
		{-128, []byte{0x80}},
		{-129, []byte{0xff, 0x7f}},
		{32767, []byte{0x7f, 0xff}},
		{32768, []byte{0x00, 0x80, 0x00}},
		{-32768, []byte{0x80, 0x00}},
		{-32769, []byte{0xff, 0x7f, 0xff}},
		{8388607, []byte{0x7f, 0xff, 0xff}},
		{-8388608, []byte{0x80, 0x00, 0x00}},
		{math.MaxInt32, []byte{0x7f, 0xff, 0xff, 0xff}},
		{math.MinInt32, []byte{0x80, 0x00, 0x00, 0x00}},
	}

	for _, test := range tests {
		encoded := EncodeVarInt(test.value)
		if !bytes.Equal(encoded, test.want) {
			t.Errorf("EncodeVarInt(%d) = %x, want %x", test.value, encoded, test.want)
		}
		if got := DecodeVarInt(encoded); got != test.value {
			t.Errorf("DecodeVarInt(%x) = %d, want %d", encoded, got, test.value)
		}
	}
}

func TestUnsigned24Encoding(t *testing.T) {
	value := Uint24(0x123456)
	encoded, err := value.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x12, 0x34, 0x56}; !bytes.Equal(encoded, want) {
		t.Fatalf("encoded Uint24 = %x, want %x", encoded, want)
	}

	var decoded Uint24
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded != value {
		t.Fatalf("decoded Uint24 = %x, want %x", decoded, value)
	}
}

func TestFixedWidthDecodersRejectWrongLength(t *testing.T) {
	tests := []struct {
		name string
		want int
		fn   func([]byte) error
	}{
		{"int8", 1, func(b []byte) error { var v Int8; return v.UnmarshalBinary(b) }},
		{"int16", 2, func(b []byte) error { var v Int16; return v.UnmarshalBinary(b) }},
		{"int24", 3, func(b []byte) error { var v Int24; return v.UnmarshalBinary(b) }},
		{"int32", 4, func(b []byte) error { var v Int32; return v.UnmarshalBinary(b) }},
		{"uint16", 2, func(b []byte) error { var v Uint16; return v.UnmarshalBinary(b) }},
		{"uint24", 3, func(b []byte) error { var v Uint24; return v.UnmarshalBinary(b) }},
		{"uint32", 4, func(b []byte) error { var v Uint32; return v.UnmarshalBinary(b) }},
	}

	for _, test := range tests {
		for _, input := range [][]byte{nil, {}, {0}, {0, 0, 0, 0, 0}} {
			if len(input) == test.want {
				continue
			}
			if err := test.fn(input); err == nil {
				t.Errorf("%s accepted %d octets", test.name, len(input))
			}
		}
	}
}

func TestTagExtendedEncoding(t *testing.T) {
	tests := []struct {
		name string
		tag  Tag
		fn   func(*Tag) []byte
		want []byte
	}{
		{"extended length", Tag{TagNumber: 2, LenValue: 300}, func(tag *Tag) []byte { return tag.EncodeTag() }, []byte{0x25, 0xfe, 0x01, 0x2c}},
		{"extended number", Tag{TagNumber: 15, LenValue: 3}, func(tag *Tag) []byte { return tag.EncodeTag() }, []byte{0xf3, 0x0f}},
		{"extended number and length", Tag{TagNumber: 15, LenValue: 300}, func(tag *Tag) []byte { return tag.EncodeContextTag() }, []byte{0xfd, 0x0f, 0xfe, 0x01, 0x2c}},
		{"opening", Tag{TagNumber: 15}, func(tag *Tag) []byte { return tag.EncodeOpeningTag() }, []byte{0xfe, 0x0f}},
		{"closing", Tag{TagNumber: 15}, func(tag *Tag) []byte { return tag.EncodeClosingTag() }, []byte{0xff, 0x0f}},
	}

	for _, test := range tests {
		encoded := test.fn(&test.tag)
		if !bytes.Equal(encoded, test.want) {
			t.Errorf("%s encoded as %x, want %x", test.name, encoded, test.want)
		}

		var decoded Tag
		if got := decoded.DecodeTag(encoded); got != len(encoded) {
			t.Errorf("%s consumed %d octets, want %d", test.name, got, len(encoded))
		}
		if decoded.TagNumber != test.tag.TagNumber {
			t.Errorf("%s tag number = %d, want %d", test.name, decoded.TagNumber, test.tag.TagNumber)
		}
	}
}

func TestTagDecodeRejectsTruncatedInput(t *testing.T) {
	inputs := [][]byte{
		nil,
		{0xf0},
		{0x05},
		{0x05, 0xfe},
		{0x05, 0xfe, 0x01},
		{0x05, 0xff, 0x00, 0x00, 0x00},
		{0xf5, 0x0f},
		{0xf5, 0x0f, 0xff, 0x00, 0x00, 0x00},
	}

	for _, input := range inputs {
		var tag Tag
		if got := tag.DecodeTag(input); got != 0 {
			t.Errorf("DecodeTag(%x) consumed %d octets", input, got)
		}
	}
}

func TestTagResetClearsFlags(t *testing.T) {
	tag := Tag{TagNumber: 4, LenValue: 10, Context: true, Extended: true, Opening: true, Closing: true}
	tag.Reset()
	if tag != (Tag{}) {
		t.Fatalf("Reset left state behind: %+v", tag)
	}
}

func TestObjectIdentifierPreservesTwentyTwoBitInstance(t *testing.T) {
	wire := []byte{0x02, 0x3f, 0xff, 0xff}
	var objectID ObjectId
	if err := objectID.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if got := objectID.InstanceNumber(); got != BacnetMaxInstance {
		t.Fatalf("instance = %d, want %d", got, BacnetMaxInstance)
	}
	encoded, err := objectID.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, wire) {
		t.Fatalf("object identifier re-encoded as %x, want %x", encoded, wire)
	}
}

func TestObjectIdRoundTrip22Bit(t *testing.T) {
	var objectID ObjectId
	objectID.Type = 0
	if err := objectID.SetInstanceNumber(70000); err != nil {
		t.Fatal(err)
	}

	encoded, err := objectID.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0x00, 0x01, 0x11, 0x70}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("object identifier encoded as %x, want %x", encoded, want)
	}

	var decoded ObjectId
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.InstanceNumber(); got != 70000 {
		t.Fatalf("instance = %d, want 70000", got)
	}
}

func TestDoubleUsesNetworkByteOrder(t *testing.T) {
	encoded, err := Double(1).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x3f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}; !bytes.Equal(encoded, want) {
		t.Fatalf("encoded double = %x, want %x", encoded, want)
	}
}

func TestBitStringPreservesUnusedBitsOctet(t *testing.T) {
	value := BitString{0x01, 0x80}
	encoded, err := value.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x00, 0x80, 0x01}; !bytes.Equal(encoded, want) {
		t.Fatalf("encoded bit string = %x, want %x", encoded, want)
	}
	var decoded BitString
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, value) {
		t.Fatalf("decoded bit string = %x, want %x", decoded, value)
	}
}

func TestDecodedStructuredValuesCanBeReencoded(t *testing.T) {
	wires := [][]byte{
		{0xa4, 124, 7, 16, 4},
		{0xb4, 12, 34, 56, 78},
		{0xc4, 0x02, 0x00, 0x00, 0x7b},
	}
	for _, wire := range wires {
		var value PropertyValue
		if err := value.UnmarshalBinary(wire); err != nil {
			t.Fatal(err)
		}
		encoded, err := value.MarshalBinary()
		if err != nil {
			t.Fatalf("re-encoding %x: %v", wire, err)
		}
		if !bytes.Equal(encoded, wire) {
			t.Fatalf("re-encoded %x as %x", wire, encoded)
		}
	}
}

func TestDevicePreservesFullInstanceNumber(t *testing.T) {
	objectID := &ObjectId{Type: ObjectTypeDevice}
	if err := objectID.SetInstanceNumber(70000); err != nil {
		t.Fatal(err)
	}
	objectBytes, err := objectID.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	wire := append([]byte{0xc4}, objectBytes...)
	wire = append(wire, 0x22, 0x05, 0xc4, 0x91, 0x03, 0x21, 0x01)
	var device Device
	if err := device.UnmarshalBinary(wire); err != nil {
		t.Fatal(err)
	}
	if device.DeviceInstance != 70000 {
		t.Fatalf("device instance = %d", device.DeviceInstance)
	}
}

func TestDeviceResetClearsOriginInterface(t *testing.T) {
	device := Device{OriginInterface: "eth0"}
	device.Reset()
	if device.OriginInterface != "" {
		t.Fatalf("origin interface survived reset: %q", device.OriginInterface)
	}
}

func TestDateStringUsesDayAndTimeRequiresExactLength(t *testing.T) {
	if got := (Date{Year: 2026, Month: 7, Day: 16, Weekday: 4}).String(); got != "2026-07-16" {
		t.Fatalf("date string = %q", got)
	}
	var value Time
	if err := value.UnmarshalBinary([]byte{1, 2, 3, 4, 5}); err == nil {
		t.Fatal("time accepted trailing octets")
	}
}

func TestMalformedTypeDecoderCorpus(t *testing.T) {
	state := uint32(0x6d2b79f5)
	for size := 0; size < 64; size++ {
		data := make([]byte, size)
		for i := range data {
			state = state*1664525 + 1013904223
			data[i] = byte(state >> 24)
		}

		var tag Tag
		tag.DecodeTag(data)
		var value PropertyValue
		_ = value.UnmarshalBinary(data)
		var property Property
		_ = property.UnmarshalBinary(data)
		var device Device
		_ = device.UnmarshalBinary(data)
		var header Header
		_ = header.UnmarshalBinary(data)
	}
}
