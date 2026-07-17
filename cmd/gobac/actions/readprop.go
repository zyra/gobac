package actions

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/v2/bacnet/types"
)

var readPropertyRequest = func(ctx context.Context, address net.IP, object types.ObjectId, propertyID types.PropertyId) ([]*types.PropertyValue, error) {
	return server.ReadObjectProperty(ctx, address, object, propertyID)
}

// parseObjectInstance parses a BACnet object instance CLI argument, accepting
// the full 22-bit instance range (0..4194303).
func parseObjectInstance(arg string) (uint32, error) {
	v, err := strconv.ParseUint(arg, 10, 32)
	if err != nil {
		return 0, err
	}
	if v > types.BacnetMaxInstance {
		return 0, fmt.Errorf("object instance %d exceeds maximum of %d", v, types.BacnetMaxInstance)
	}
	return uint32(v), nil
}

func ReadProp(ctx *cli.Context) (err error) {
	if len(ctx.Args()) < 4 {
		msg := fmt.Sprintf("Invalid number of arguments. Expected 4 and got %d.\n", len(ctx.Args()))
		return cli.NewExitError(msg, 1)
	}

	var objectType types.Uint16
	var address net.IP
	var propertyId types.PropertyId
	var object types.ObjectId

	address = net.ParseIP(ctx.Args().Get(0))

	if v, e := strconv.Atoi(ctx.Args().Get(1)); e != nil {
		return e
	} else {
		objectType = types.Uint16(v)
	}

	instance, e := parseObjectInstance(ctx.Args().Get(2))
	if e != nil {
		return e
	}

	object.Type = objectType
	if e := object.SetInstanceNumber(instance); e != nil {
		return e
	}

	if v, e := strconv.Atoi(ctx.Args().Get(3)); e != nil {
		return e
	} else {
		propertyId = types.PropertyId(v)
	}

	logVerbosef("Reading property %d on object %d instance %d...\n", propertyId, objectType, object.InstanceNumber())

	prop, err := readPropertyRequest(context.TODO(), address, object, propertyId)

	if err != nil {
		return err
	}

	valueLen := len(prop)

	fmt.Printf("Received %d value(s)\n\n", valueLen)

	if valueLen == 0 {
		return
	}

	for i, v := range prop {
		fmt.Printf("%d. [ Type: %d ]  [ Value: %s ]\n", i+1, v.Type, v.ReadAsString())
	}

	return
}
