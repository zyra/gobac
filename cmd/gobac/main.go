package main

import (
	"github.com/urfave/cli"
	"github.com/zyra/gobac/cmd/gobac/actions"
	"log"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "gobac cli"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
		{
			Name:    "whois",
			Aliases: []string{"wi"},
			Usage:   "Send a who-is broadcast and listen for responses",
			Action:  actions.Whois,
		},
		{
			Name:      "readprop",
			Aliases:   []string{"rp"},
			Usage:     "Read an object property",
			Action:    actions.ReadProp,
			ArgsUsage: "address object-id object-instance property-id [index]",
		},
		{
			Name:      "writeprop",
			Aliases:   []string{"wp"},
			Usage:     "Write an object property",
			ArgsUsage: "address object-id object-instance property-id tag value [index]",
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "priority",
					Value: 8,
					Usage: "value priority",
				},
			},
			Action: actions.WritePropAction,
		},
		{
			Name:    "scan",
			Aliases: []string{"s"},
			Usage:   "Scan the network thoroughly and get everything that exists",
			Action:  actions.Scan,
		},
	}

	app.Before = actions.Before

	app.Flags = []cli.Flag{
		cli.Float64Flag{
			Name:  "timeout, t",
			Value: 3,
			Usage: "timeout for requests in seconds",
		},
		cli.UintFlag{
			Name:  "port, p",
			Value: 0xBAC0,
			Usage: "bacnet port",
		},
		cli.UintFlag{
			Name:  "server-port, s",
			Value: 0xBAC0,
			Usage: "server BBMD port",
		},
		cli.StringFlag{
			Name:  "interface, i",
			Value: "eno0",
			Usage: "interface name to bind to",
		},
		cli.UintFlag{
			Name:  "concurrency, c",
			Value: 5,
			Usage: "concurrent listeners to run",
		},
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "verbose logging",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
