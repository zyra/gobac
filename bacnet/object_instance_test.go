package bacnet

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestReadObjectPropertyRejectsBadObject(t *testing.T) {
	s := &Server{}

	// Building an ObjectId with an instance above the 22-bit range is
	// rejected at construction time with an error naming the maximum.
	var badInstance types.ObjectId
	err := badInstance.SetInstanceNumber(0x400000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "4194303")

	// ReadObjectProperty independently rejects an object whose type is
	// outside the 10-bit object type range.
	_, err = s.ReadObjectProperty(context.Background(), net.IPv4(192, 0, 2, 1), types.ObjectId{Type: 2000}, 85)
	require.Error(t, err)
	require.Contains(t, err.Error(), "1023")

	// A nil device IP is rejected regardless of the object.
	_, err = s.ReadObjectProperty(context.Background(), nil, types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 1}, 85)
	require.EqualError(t, err, "received a nil or empty device IP")
}

func TestReadPropertyLegacyAndObjectVariantEncodeSame(t *testing.T) {
	legacyRequest := &pdu.ReadPropertyPdu{
		Property: &types.Property{
			ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogInput, Instance: 42},
			ID:       85,
		},
	}

	var object types.ObjectId
	require.NoError(t, object.SetInstanceNumber(42))
	object.Type = types.ObjectTypeAnalogInput

	objectRequest := &pdu.ReadPropertyPdu{
		Property: &types.Property{
			ObjectId: &object,
			ID:       85,
		},
	}

	legacyBytes, err := legacyRequest.MarshalBinary()
	require.NoError(t, err)

	objectBytes, err := objectRequest.MarshalBinary()
	require.NoError(t, err)

	require.Equal(t, legacyBytes, objectBytes)
}
