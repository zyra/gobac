package actions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/v2/bacnet/types"
)

var writePropertyRequest = func(ctx context.Context, address net.IP, object types.ObjectId, propertyID types.PropertyId, tag types.DataType, priority uint8, value interface{}) error {
	return server.WriteObjectProperty(ctx, address, object, propertyID, tag, priority, value)
}

func WritePropAction(ctx *cli.Context) (err error) {
	if len(ctx.Args()) < 5 {
		msg := fmt.Sprintf("Invalid number of arguments. Expected 5-6 and got %d.\n", len(ctx.Args()))
		return cli.NewExitError(msg, 1)
	}

	var objectType types.Uint16
	var address net.IP
	var propertyId types.PropertyId
	var tag types.DataType
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

	if v, e := strconv.Atoi(ctx.Args().Get(4)); e != nil {
		return e
	} else {
		tag = types.DataType(v)
	}

	val := ctx.Args().Get(5)
	var parsedVal interface{}

	switch tag {
	case types.TagBoolean:
		parsedVal, err = strconv.ParseBool(val)

	case types.TagUnsigned, types.TagEnumerated, types.TagSigned:
		parsedVal, err = strconv.Atoi(val)

	case types.TagReal:
		parsedVal, err = strconv.ParseFloat(val, 32)

	case types.TagDouble:
		parsedVal, err = strconv.ParseFloat(val, 64)

	case types.TagCharacterString:
		parsedVal = types.CharacterString{
			Value:    val,
			Encoding: 0,
		}

	case types.TagNull:
		parsedVal = nil

	default:
		return errors.New("unsupported data type (by the CLI)")
	}

	if err != nil {
		return err
	}

	priority := uint8(ctx.Uint("priority"))

	logVerbosef("Writing property %d on object %d instance %d...\n", propertyId, objectType, object.InstanceNumber())

	if err := writePropertyRequest(context.TODO(), address, object, propertyId, tag, priority, parsedVal); err != nil {
		return err
	} else {
		fmt.Println("Write was successful!")
	}
	return
}
