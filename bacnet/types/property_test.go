package types

import (
	"reflect"
	"testing"
)

var prop = Property{
	ObjectId: &ObjectId{
		Type:     1,
		Instance: 1,
	},
	ID: 5,
	Values: []*PropertyValue{
		{
			Type:  TagEnumerated,
			Value: 1,
		},
	},
}

var values = []PropertyValue{
	{
		Type:  TagSigned,
		Value: 1,
	},
	{
		Type:  TagReal,
		Value: 5.2,
	},
	{
		Type:  TagDouble,
		Value: 5.2,
	},
	{
		Type: TagDate,
		Value: &Date{
			Year:    2018,
			Month:   8,
			Day:     8,
			Weekday: WeekdaySunday,
		},
	},
	{
		Type: TagCharacterString,
		Value: CharacterString{
			Encoding: 0,
			Value:    "hello world this is pretty cool",
		},
	},
}

var binaries [][]byte

var propBinary []byte

func TestProperty_MarshalBinary(t *testing.T) {
	b, e := prop.MarshalBinary()

	if e != nil {
		t.Error(e)
		t.FailNow()
	}

	propBinary = b
}

func TestProperty_UnmarshalBinary(t *testing.T) {
	if propBinary == nil {
		return
	}

	prop2 := &Property{}
	err := prop2.UnmarshalBinary(propBinary)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !reflect.DeepEqual(prop.ObjectId, prop2.ObjectId) {
		t.Error("object ids don't match")
		t.FailNow()
	}

	if !reflect.DeepEqual(prop.Values[0], prop2.Values[0]) {
		t.Error("value doesn't match")
		t.FailNow()
	}

	if prop.ID != prop2.ID {
		t.Error("prop IDs don't match")
		t.FailNow()
	}
}

func BenchmarkPropertyValue_MarshalBinary(b *testing.B) {
	binaries = make([][]byte, len(values))
	var bb []byte
	var e error

	for i, v := range values {
		for k := 0; k < 1e4; k++ {
			bb, e = v.MarshalBinary()
		}

		if e != nil {
			b.Error(e)
			b.FailNow()
		}

		binaries[i] = bb
	}
}

func BenchmarkPropertyValue_UnmarshalBinary(b *testing.B) {
	val := PropertyValue{}

	var e error
	for _, v := range binaries {
		for k := 0; k < 1e4; k++ {
			e = val.UnmarshalBinary(v)
		}

		if e != nil {
			b.Error(e)
			b.FailNow()
		}
	}
}
