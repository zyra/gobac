package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli"
	"github.com/zyra/gobac/bacnet"
)

var server *bacnet.Server
var verbose bool

func logVerbose(vals ...interface{}) {
	if !verbose {
		return
	}

	fmt.Println(vals...)
}

func logVerbosef(format string, vals ...interface{}) {
	if !verbose {
		return
	}

	fmt.Printf(format+"\n", vals...)
}

func Before(c *cli.Context) error {
	if c.NArg() == 0 || hasHelpArgument(c.Args()) {
		return nil
	}

	verbose = c.GlobalBool("verbose")

	serverConfig := bacnet.NewServerConfig()
	serverConfig.SetDefaultTimeout(time.Duration(c.GlobalFloat64("timeout")*1000) * time.Millisecond)
	serverConfig.SetInterfaceName(c.GlobalString("interface"))
	serverConfig.SetReceiveErrors(true)
	serverConfig.SetListenPort(uint16(c.GlobalUint("port")))
	serverConfig.SetServerBBMDPort(uint16(c.GlobalUint("server-port")))

	logVerbosef("Starting new server on interface %s and port %d\n", serverConfig.InterfaceName, serverConfig.ServerBBMDPort)

	s, err := bacnet.NewServer(serverConfig)

	if err != nil {
		return err
	}

	server = s

	go s.Listen(context.Background())

	<-s.Start()

	return nil
}

func hasHelpArgument(args cli.Args) bool {
	if len(args) == 0 || args[0] == "help" {
		return true
	}
	return len(args) > 1 && (args[1] == "--help" || args[1] == "-h")
}
