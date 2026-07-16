package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/v2/cmd/gobac/actions"
)

var version = "dev"

func main() {
	app := newApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = "gobac cli"
	app.Version = version

	app.Commands = []cli.Command{
		{
			Name:    "whois",
			Aliases: []string{"wi"},
			Usage:   "Send a who-is broadcast and listen for responses",
			Action:  actions.Whois,
			Flags: []cli.Flag{
				cli.Float64Flag{
					Name:  "duration, d",
					Value: 3,
					Usage: "how long to listen for broadcasts for",
				},
			},
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
					Value: 9,
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
			Value: 5,
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
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "verbose logging",
		},
		cli.BoolFlag{
			Name:  "json, j",
			Usage: "output json",
		},
	}

	return app
}
