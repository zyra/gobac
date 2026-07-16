package types

import (
	"bytes"
	"testing"
)

func TestPropertyValueOctetStringLength(t *testing.T) {
	value := &PropertyValue{Type: TagOctetString, Value: []byte{1, 2, 3, 4, 5, 6}}
	encoded, err := value.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x65, 0x06, 1, 2, 3, 4, 5, 6}; !bytes.Equal(encoded, want) {
		t.Fatalf("octet string encoded as %x, want %x", encoded, want)
	}

	var decoded PropertyValue
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Value.([]byte), value.Value.([]byte)) {
		t.Fatalf("decoded octet string = %x", decoded.Value)
	}
}

func TestPropertyEncodesEveryValue(t *testing.T) {
	property := &Property{
		ObjectId: &ObjectId{Type: 1, Instance: 1},
		ID:       85,
		Values: []*PropertyValue{
			{Type: TagUnsigned, Value: uint32(1)},
			{Type: TagUnsigned, Value: uint32(2)},
		},
	}
	encoded, err := property.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x0c, 0x00, 0x40, 0x00, 0x01, 0x19, 0x55, 0x3e, 0x21, 0x01, 0x21, 0x02, 0x3f}; !bytes.Equal(encoded, want) {
		t.Fatalf("property encoded as %x, want %x", encoded, want)
	}

	var decoded Property
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Values) != 2 {
		t.Fatalf("decoded %d values, want 2", len(decoded.Values))
	}
}

func TestPropertyArrayIndexEncoding(t *testing.T) {
	tests := []struct {
		name  string
		index uint32
		set   bool
		want  []byte
	}{
		{"omitted", 0, false, []byte{0x0c, 0x00, 0x40, 0x00, 0x01, 0x19, 0x55}},
		{"zero", 0, true, []byte{0x0c, 0x00, 0x40, 0x00, 0x01, 0x19, 0x55, 0x29, 0x00}},
		{"one", 1, true, []byte{0x0c, 0x00, 0x40, 0x00, 0x01, 0x19, 0x55, 0x29, 0x01}},
	}

	for _, test := range tests {
		property := &Property{
			ObjectId: &ObjectId{Type: 1, Instance: 1},
			ID:       85,
			Index:    test.index,
			HasIndex: test.set,
		}
		encoded, err := property.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, test.want) {
			t.Errorf("%s index encoded as %x, want %x", test.name, encoded, test.want)
		}
	}
}

func TestPropertyRejectsMalformedInput(t *testing.T) {
	inputs := [][]byte{
		nil,
		{},
		{0x0c},
		{0x0c, 0, 0, 0, 1},
		{0x0c, 0, 0, 0, 1, 0x19},
		{0x0c, 0, 0, 0, 1, 0x19, 85, 0x3e, 0x25},
		{0x0c, 0, 0, 0, 1, 0x19, 85, 0x3e, 0x21, 1},
	}
	for _, input := range inputs {
		var property Property
		if err := property.UnmarshalBinary(input); err == nil {
			t.Errorf("UnmarshalBinary(%x) succeeded", input)
		}
	}
}

func TestPropertyValueRejectsTruncatedInput(t *testing.T) {
	inputs := [][]byte{{}, {0x44, 0, 0, 0}, {0x55, 0xff, 0, 0, 0, 4, 0, 0}, {0xc4, 0, 0, 0}}
	for _, input := range inputs {
		var value PropertyValue
		if err := value.UnmarshalBinary(input); err == nil {
			t.Errorf("UnmarshalBinary(%x) succeeded", input)
		}
	}
}
