package actions

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/v2/bacnet/types"
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

	require.NoError(t, Before(actionContext("readprop", "--help")))
	require.Nil(t, server)
	require.False(t, hasHelpArgument(cli.Args{"writeprop", "address", "2", "1", "85", "7", "help"}))
}

func TestWhoisSortsAndPreserves22BitInstances(t *testing.T) {
	previous := whoIsRequest
	defer func() { whoIsRequest = previous }()

	makeDevice := func(instance uint32, n byte) *types.Device {
		d := &types.Device{}
		d.DeviceInstance = instance
		d.IPAddress = net.IPv4(192, 0, 2, n)
		return d
	}

	ch := make(chan *types.Device, 3)
	ch <- makeDevice(4194303, 1) // 0x3FFFFF, max 22-bit value
	ch <- makeDevice(70000, 2)   // above the old 16-bit ceiling of 65535
	ch <- makeDevice(12, 3)
	close(ch)

	whoIsRequest = func(_ context.Context, _ time.Duration) (<-chan *types.Device, error) {
		return ch, nil
	}

	devices, err := whois(time.Second)

	require.NoError(t, err)
	require.Len(t, devices, 3)
	require.Equal(t, []uint32{12, 70000, 4194303}, []uint32{
		devices[0].DeviceInstance,
		devices[1].DeviceInstance,
		devices[2].DeviceInstance,
	})
}

func TestWhoisReturnsErrorWhenNoDevicesFound(t *testing.T) {
	previous := whoIsRequest
	defer func() { whoIsRequest = previous }()

	ch := make(chan *types.Device)
	close(ch)

	whoIsRequest = func(_ context.Context, _ time.Duration) (<-chan *types.Device, error) {
		return ch, nil
	}

	_, err := whois(time.Second)

	require.EqualError(t, err, "no devices found")
}

func TestWhoisActionForwardsDuration(t *testing.T) {
	previous := whoIsRequest
	defer func() { whoIsRequest = previous }()

	var gotTimeout time.Duration

	ch := make(chan *types.Device, 1)
	d := &types.Device{}
	d.DeviceInstance = 1
	d.IPAddress = net.IPv4(192, 0, 2, 1)
	ch <- d
	close(ch)

	whoIsRequest = func(_ context.Context, timeout time.Duration) (<-chan *types.Device, error) {
		gotTimeout = timeout
		return ch, nil
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.Float64("duration", 2.5, "")
	ctx := cli.NewContext(nil, set, nil)

	err := Whois(ctx)

	require.NoError(t, err)
	require.Equal(t, 2500*time.Millisecond, gotTimeout)
}

func TestScanReturnsWhoisError(t *testing.T) {
	previous := whoIsRequest
	defer func() { whoIsRequest = previous }()

	whoIsRequest = func(_ context.Context, _ time.Duration) (<-chan *types.Device, error) {
		return nil, errors.New("boom")
	}

	err := Scan(actionContext())

	require.EqualError(t, err, "boom")
}

func TestScanCollectsObjectsAndProperties(t *testing.T) {
	previousWhoIs := whoIsRequest
	previousDeviceObjects := deviceObjects
	previousObjectProperties := objectProperties
	defer func() {
		whoIsRequest = previousWhoIs
		deviceObjects = previousDeviceObjects
		objectProperties = previousObjectProperties
	}()

	scanProperties := []*types.Property{
		{
			ID: types.PropertyObjectName,
			Values: []*types.PropertyValue{
				{Type: types.TagCharacterString, Value: types.CharacterString{Value: "AHU"}},
			},
		},
		{
			ID: types.PropertyDescription,
			Values: []*types.PropertyValue{
				{Type: types.TagCharacterString, Value: types.CharacterString{Value: "desc"}},
			},
		},
		{
			ID: types.PropertyPresentValue,
			Values: []*types.PropertyValue{
				{Type: types.TagReal, Value: types.Real(21.5)},
			},
		},
	}

	ch := make(chan *types.Device, 1)
	dev := &types.Device{}
	dev.DeviceInstance = 70000
	dev.IPAddress = net.IPv4(192, 0, 2, 10)
	ch <- dev
	close(ch)

	whoIsRequest = func(_ context.Context, _ time.Duration) (<-chan *types.Device, error) {
		return ch, nil
	}

	var mu sync.Mutex
	var gotDeviceInstance uint32

	deviceObjects = func(device types.Device) ([]*types.Object, error) {
		mu.Lock()
		gotDeviceInstance = device.DeviceInstance
		mu.Unlock()

		o := &types.Object{IPAddress: net.IPv4(192, 0, 2, 11)}
		return []*types.Object{o}, nil
	}

	objectProperties = func(_ types.Object) ([]*types.Property, error) {
		return scanProperties, nil
	}

	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := Scan(actionContext())

	w.Close()
	os.Stdout = old

	out, readErr := ioutil.ReadAll(r)
	require.NoError(t, readErr)

	require.NoError(t, err)

	mu.Lock()
	require.Equal(t, uint32(70000), gotDeviceInstance)
	mu.Unlock()

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &result))
	require.Len(t, result, 1)

	objects, ok := result[0]["Objects"].([]interface{})
	require.True(t, ok)
	require.Len(t, objects, 1)

	obj, ok := objects[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "AHU", obj["Name"])
	require.Equal(t, "desc", obj["Description"])
	require.Equal(t, 21.5, obj["FloatValue"])
}
