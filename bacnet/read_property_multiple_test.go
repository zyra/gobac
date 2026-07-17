package bacnet

import (
	"context"
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestReadPropertyMultipleValidation(t *testing.T) {
	s := &Server{}

	specs := []ReadAccessSpec{
		{
			Object:     types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 1},
			Properties: []ReadAccessSpecProperty{{ID: 85}},
		},
	}

	_, err := s.ReadPropertyMultiple(context.Background(), nil, specs)
	if err == nil || err.Error() != "received a nil or empty device IP" {
		t.Fatalf("nil IP error = %v, want %q", err, "received a nil or empty device IP")
	}

	_, err = s.ReadPropertyMultiple(context.Background(), net.IPv4(192, 0, 2, 1), nil)
	if err == nil || err.Error() != "received no read access specifications" {
		t.Fatalf("empty specs error = %v, want %q", err, "received no read access specifications")
	}
}
