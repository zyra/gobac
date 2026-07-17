package bacnet

import (
	"context"
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestWritePropertyMultipleValidation(t *testing.T) {
	s := &Server{}

	specs := []WriteAccessSpec{
		{
			Object: types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1},
			Properties: []WriteAccessSpecProperty{
				{ID: types.PropertyPresentValue, Tag: types.TagReal, Value: float32(21)},
			},
		},
	}

	err := s.WritePropertyMultiple(context.Background(), nil, specs)
	if err == nil || err.Error() != "received a nil or empty device IP" {
		t.Fatalf("nil IP error = %v, want %q", err, "received a nil or empty device IP")
	}

	err = s.WritePropertyMultiple(context.Background(), net.IPv4(192, 0, 2, 1), nil)
	if err == nil || err.Error() != "received no write access specifications" {
		t.Fatalf("empty specs error = %v, want %q", err, "received no write access specifications")
	}
}

func TestWritePropertyMultipleErrorMessage(t *testing.T) {
	object := &types.ObjectId{Type: types.ObjectTypeBinaryValue, Instance: 2}
	err := &WritePropertyMultipleError{
		ErrorClass:          types.ErrorClassProperty,
		ErrorCode:           types.ErrorCodeWriteAccessDenied,
		FirstFailedObjectId: object,
		FirstFailedProperty: types.PropertyPresentValue,
	}
	want := "property " + types.ErrorCode(types.ErrorCodeWriteAccessDenied).String()
	if err.Error() != want {
		t.Fatalf("error message = %q, want %q", err.Error(), want)
	}
}
