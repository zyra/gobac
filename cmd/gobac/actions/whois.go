package actions

import (
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
	"time"
)

func Whois(ctx *cli.Context) (err error) {
	duration := time.Duration(ctx.Float64("duration")*1000) * time.Millisecond

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

	devices = make([]*types.Device, 0)

	dChan, err := server.WhoIs(duration)

	if err != nil {
		return nil, err
	}

	for {
		dev, open := <-dChan

		if dev != nil {
			devices = append(devices, dev)
			logVerbosef("Found a new device with instance %d at %s", dev.ObjectId.Instance, dev.IPAddress)
		}

		if !open {
			break
		}
	}

	lenDevices := len(devices)

	fmt.Printf("Found %d devices\n\n", lenDevices)

	if lenDevices == 0 {
		return nil, errors.New("no devices found")
	}

	return devices, nil
}
