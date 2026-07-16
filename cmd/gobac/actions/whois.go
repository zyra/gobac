package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet/types"
)

func Whois(ctx *cli.Context) (err error) {
	duration := time.Duration(ctx.Float64("duration")*1000) * time.Millisecond

	devices, err := whois(duration)

	if err != nil {
		return err
	}

	if ctx.GlobalBool("json") {
		if j, _ := json.Marshal(devices); j != nil {
			fmt.Println(string(j))
			return
		}
	}

	for i, d := range devices {
		fmt.Printf("%d. [ Device ID: %d ]  [ IPAddress: %s ]\n", i+1, d.ObjectId.Instance, d.IPAddress.String())
	}

	return
}

func whois(duration time.Duration) (devices []*types.Device, err error) {
	logVerbose("Sending whois request")

	devices = make([]*types.Device, 0)

	dChan, err := server.WhoIs(context.TODO(), duration)

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

	logVerbosef("Found %d devices\n\n", lenDevices)

	if lenDevices == 0 {
		return nil, errors.New("no devices found")
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].DeviceID < devices[j].DeviceID
	})

	return devices, nil
}
