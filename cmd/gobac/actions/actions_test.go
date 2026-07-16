package actions

import (
	"context"
	"flag"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
)

func actionContext(args ...string) *cli.Context {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.Uint("priority", 9, "")
	_ = set.Parse(args)
	return cli.NewContext(nil, set, nil)
}

func TestReadPropRejectsTooFewArguments(t *testing.T) {
	err := ReadProp(actionContext("192.0.2.1", "0", "1"))

	require.Error(t, err)
	exitErr, ok := err.(cli.ExitCoder)
	require.True(t, ok)
	require.Equal(t, 1, exitErr.ExitCode())
}

func TestReadPropRejectsInvalidObjectType(t *testing.T) {
	err := ReadProp(actionContext("192.0.2.1", "not-a-number", "1", "85"))

	require.Error(t, err)
}

func TestReadPropForwardsParsedArguments(t *testing.T) {
	previous := readPropertyRequest
	defer func() { readPropertyRequest = previous }()

	var address net.IP
	var objectType, objectInstance types.Uint16
	var propertyID types.PropertyId
	readPropertyRequest = func(_ context.Context, gotAddress net.IP, gotObjectType, gotObjectInstance types.Uint16, gotPropertyID types.PropertyId) ([]*types.PropertyValue, error) {
		address = append(net.IP(nil), gotAddress...)
		objectType = gotObjectType
		objectInstance = gotObjectInstance
		propertyID = gotPropertyID
		return nil, nil
	}

	err := ReadProp(actionContext("192.0.2.10", "2", "17", "85"))

	require.NoError(t, err)
	require.True(t, address.Equal(net.ParseIP("192.0.2.10")))
	require.Equal(t, types.Uint16(2), objectType)
	require.Equal(t, types.Uint16(17), objectInstance)
	require.Equal(t, types.PropertyId(85), propertyID)
}

func TestWritePropRejectsTooFewArguments(t *testing.T) {
	err := WritePropAction(actionContext("192.0.2.1", "2", "1", "85"))

	require.Error(t, err)
	exitErr, ok := err.(cli.ExitCoder)
	require.True(t, ok)
	require.Equal(t, 1, exitErr.ExitCode())
}

func TestWritePropRejectsUnsupportedDataType(t *testing.T) {
	err := WritePropAction(actionContext("192.0.2.1", "2", "1", "85", "255", "value"))

	require.EqualError(t, err, "unsupported data type (by the CLI)")
}

func TestWritePropForwardsParsedArguments(t *testing.T) {
	previous := writePropertyRequest
	defer func() { writePropertyRequest = previous }()

	var address net.IP
	var objectType, objectInstance types.Uint16
	var propertyID types.PropertyId
	var tag types.DataType
	var priority uint8
	var value interface{}
	writePropertyRequest = func(_ context.Context, gotAddress net.IP, gotObjectType, gotObjectInstance types.Uint16, gotPropertyID types.PropertyId, gotTag types.DataType, gotPriority uint8, gotValue interface{}) error {
		address = append(net.IP(nil), gotAddress...)
		objectType = gotObjectType
		objectInstance = gotObjectInstance
		propertyID = gotPropertyID
		tag = gotTag
		priority = gotPriority
		value = gotValue
		return nil
	}

	err := WritePropAction(actionContext("192.0.2.10", "2", "17", "85", "1", "true"))

	require.NoError(t, err)
	require.True(t, address.Equal(net.ParseIP("192.0.2.10")))
	require.Equal(t, types.Uint16(2), objectType)
	require.Equal(t, types.Uint16(17), objectInstance)
	require.Equal(t, types.PropertyId(85), propertyID)
	require.Equal(t, types.DataType(types.TagBoolean), tag)
	require.Equal(t, uint8(9), priority)
	require.Equal(t, true, value)
}

func TestBeforeDoesNotStartServerForHelp(t *testing.T) {
	previous := server
	defer func() { server = previous }()

	server = nil
	require.NoError(t, Before(actionContext("help")))
	require.Nil(t, server)
}
