package pdu

import (
	"bytes"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestTimeSyncPduRoundTrip(t *testing.T) {
	request := &TimeSyncPdu{
		Date: types.Date{Year: 2026, Month: 7, Day: 17, Weekday: types.WeekdayFriday},
		Time: types.Time{Hour: 14, Min: 30, Sec: 15, Hundredths: 25},
	}

	encoded, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0xa4, 0x7e, 0x07, 0x11, 0x05, 0xb4, 0x0e, 0x1e, 0x0f, 0x19}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded time sync = % x, want % x", encoded, want)
	}

	decoded := &TimeSyncPdu{}
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if *decoded != *request {
		t.Fatalf("decoded = %+v, want %+v", decoded, request)
	}
}

func TestTimeSyncPduUnmarshalRejectsWrongValueCount(t *testing.T) {
	dateOnly := types.PropertyValue{Type: types.TagDate, Value: types.Date{Year: 2026, Month: 7, Day: 17, Weekday: types.WeekdayFriday}}
	encoded, err := dateOnly.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	decoded := &TimeSyncPdu{}
	if err := decoded.UnmarshalBinary(encoded); err == nil {
		t.Fatal("expected an error when only a date is present")
	}
}

func TestTimeSyncPduUnmarshalRejectsWrongTagOrder(t *testing.T) {
	timeVal := types.PropertyValue{Type: types.TagTime, Value: types.Time{Hour: 14, Min: 30, Sec: 15, Hundredths: 25}}
	timeBytes, err := timeVal.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	dateVal := types.PropertyValue{Type: types.TagDate, Value: types.Date{Year: 2026, Month: 7, Day: 17, Weekday: types.WeekdayFriday}}
	dateBytes, err := dateVal.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	decoded := &TimeSyncPdu{}
	if err := decoded.UnmarshalBinary(append(timeBytes, dateBytes...)); err == nil {
		t.Fatal("expected an error when time precedes date")
	}
}
