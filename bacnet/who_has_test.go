package bacnet

import (
	"context"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestWhoHasValidation(t *testing.T) {
	s := &Server{}

	_, err := s.WhoHas(context.Background(), time.Second, WhoHasQuery{
		ObjectId:   &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 3},
		ObjectName: "pump",
	})
	if err == nil || err.Error() != "exactly one of ObjectId or ObjectName must be set" {
		t.Fatalf("both-set error = %v, want %q", err, "exactly one of ObjectId or ObjectName must be set")
	}

	_, err = s.WhoHas(context.Background(), time.Second, WhoHasQuery{})
	if err == nil || err.Error() != "exactly one of ObjectId or ObjectName must be set" {
		t.Fatalf("neither-set error = %v, want %q", err, "exactly one of ObjectId or ObjectName must be set")
	}
}
