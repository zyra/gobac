package actions

import (
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
	"time"
)

func Whois(ctx *cli.Context) (err error) {
	duration := time.Duration(ctx.GlobalFloat64("duration")) * 1000 * time.Millisecond

	devices, err := whois(duration)

	if err != nil {
		return err
	}

	for i, d := range devices {
		fmt.Printf("%d. [ Device ID: %d ]  [ IPAddress: %s ]\n", i+1, d.ObjectId.Instance, d.IPAddress.String())
	}

	return
}

func whois(duration time.Duration) (devices []*types.Device, err error) {
	logVerbose("Sending whois request")

	devices, err = server.WhoIs(duration)

	if err != nil {
		return nil, err
	}

	lenDevices := len(devices)

	fmt.Printf("Found %d devices\n\n", lenDevices)

	if lenDevices == 0 {
		return nil, errors.New("no devices found")
	}

	return devices, nil
}
