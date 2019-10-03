package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
	"net"
	"strconv"
)

func WritePropAction(ctx *cli.Context) (err error) {
	if len(ctx.Args()) < 5 {
		msg := fmt.Sprintf("Invalid number of arguments. Expected 5-6 and got %d.\n", len(ctx.Args()))
		return cli.NewExitError(msg, 1)
	}

	var objectType, objectInstance types.Uint16
	var address net.IP
	var propertyId types.PropertyId
	var tag types.DataType

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

	logVerbosef("Writing property %d on object %d instance %d...\n", propertyId, objectType, objectInstance)

	if err := server.WriteProperty(context.TODO(), &address, objectType, objectInstance, propertyId, tag, priority, parsedVal); err != nil {
		return err
	} else {
		fmt.Println("Write was successful!")
	}
	return
}
