package actions

import (
	"context"
	"fmt"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"strconv"
)

func ReadProp(ctx *cli.Context) (err error) {
	if len(ctx.Args()) < 4 {
		msg := fmt.Sprintf("Invalid number of arguments. Expected 4 and got %d.\n", len(ctx.Args()))
		return cli.NewExitError(msg, 1)
	}

	var objectType, objectInstance types.Uint16
	var address net.IP
	var propertyId types.PropertyId

	address = net.ParseIP(ctx.Args().Get(0))

	if v, e := strconv.Atoi(ctx.Args().Get(1)); e != nil {
		return e
	} else {
		objectType = types.Uint16(v)
	}

	if v, e := strconv.Atoi(ctx.Args().Get(2)); e != nil {
		return e
	} else {
		objectInstance = types.Uint16(v)
	}

	if v, e := strconv.Atoi(ctx.Args().Get(3)); e != nil {
		return e
	} else {
		propertyId = types.PropertyId(v)
	}

	logVerbosef("Reading property %d on object %d instance %d...\n", propertyId, objectType, objectInstance)

	prop, err := server.ReadProperty(context.TODO(), &address, objectType, objectInstance, propertyId)

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
