package ui

import (
	"testing"

	"github.com/zyra/gobac/gui/internal/session"
)

func TestParseWriteValueExact(t *testing.T) {
	cases := []struct {
		name    string
		tag     uint8
		text    string
		want    interface{}
		wantErr bool
	}{
		{"real", 4, "21.5", float32(21.5), false},
		{"boolean", 1, "true", true, false},
		{"enumerated", 9, "2", 2, false},
		{"real invalid", 4, "abc", nil, true},
		{"null ignores text", 0, "whatever", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseWriteValue(c.tag, c.text)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseWriteValue(%d, %q) = %#v, nil, want an error", c.tag, c.text, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWriteValue(%d, %q) unexpected error: %v", c.tag, c.text, err)
			}
			if got != c.want {
				t.Errorf("ParseWriteValue(%d, %q) = %#v, want %#v", c.tag, c.text, got, c.want)
			}
		})
	}
}

func TestFormatValueExact(t *testing.T) {
	cases := []struct {
		name string
		v    session.Value
		want string
	}{
		{"real", session.Value{Tag: 4, Value: float32(21.5)}, "21.5"},
		{"double", session.Value{Tag: 5, Value: float64(42.25)}, "42.25"},
		{"bit string", session.Value{Tag: 8, Value: []bool{false, false, false, true}}, "{false false false true}"},
		{"object id", session.Value{Tag: 12, Value: session.ObjectRef{Type: 2, Instance: 5}}, "2:5"},
		{"character string", session.Value{Tag: 7, Value: "hello"}, "hello"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatValue(c.v); got != c.want {
				t.Errorf("FormatValue(%+v) = %q, want %q", c.v, got, c.want)
			}
		})
	}
}
