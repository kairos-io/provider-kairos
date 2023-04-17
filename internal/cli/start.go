package cli

import (
	"fmt"
	"github.com/kairos-io/provider-kairos/internal/register"
	"os"
	"strconv"

	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v1"

	"github.com/mudler/edgevpn/pkg/node"
)

// do not edit version here, it is set by LDFLAGS
// -X 'github.com/kairos-io/provider-kairos/internal/cli.VERSION=$VERSION'
// see Earthlfile.
var VERSION = "0.0.0"
var Author = "Ettore Di Giacinto"

var networkAPI = []cli.Flag{
	&cli.StringFlag{
		Name:  "api",
		Usage: "API Address",
		Value: "http://localhost:8080",
	},
	&cli.StringFlag{
		Name:  "network-id",
		Value: "kairos",
		Usage: "Kubernetes Network Deployment ID",
	},
}

const recoveryAddr = "127.0.0.1:2222"

func Start() error {
	app := &cli.App{
		Name:    "kairos-agent-provider",
		Version: VERSION,
		Authors: []*cli.Author{
			{
				Name: Author,
			},
		},
		Usage: "kairos CLI to bootstrap, upgrade, connect and manage a kairos network",
		Description: `
The kairos CLI can be used to manage a kairos box and perform all day-two tasks, like:
- register a node (WARNING: this command will be deprecated in the next release, use the kairos-register binary instead)
- connect to a node in recovery mode
- to establish a VPN connection
- set, list roles
- interact with the network API

and much more.

For all the example cases, see: https://docs.kairos.io .
`,
		UsageText: ``,
		Copyright: Author,
		Commands: []*cli.Command{
			{
				Name:      "recovery-ssh-server",
				UsageText: "recovery-ssh-server",
				Usage:     "Starts SSH recovery service",
				Description: `
				Spawn up a simple standalone ssh server over p2p
		`,
				ArgsUsage: "Spawn up a simple standalone ssh server over p2p",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "token",
						EnvVars: []string{"TOKEN"},
					},
					&cli.StringFlag{
						Name:    "service",
						EnvVars: []string{"SERVICE"},
					},
					&cli.StringFlag{
						Name:    "password",
						EnvVars: []string{"PASSWORD"},
					},
					&cli.StringFlag{
						Name:    "listen",
						EnvVars: []string{"LISTEN"},
						Value:   recoveryAddr,
					},
				},
				Action: func(c *cli.Context) error {
					return StartRecoveryService(c.String("token"), c.String("service"), c.String("password"), c.String("listen"))
				},
			},
			register.Command(true),
			BridgeCmd(true),
			&GetKubeConfigCMD,
			&RoleCMD,
			{
				Name:      "create-config",
				Aliases:   []string{"c"},
				UsageText: "Create a config with a generated network token",

				Usage: "Creates a pristine config file",
				Description: `
		Prints a vanilla YAML configuration on screen which can be used to bootstrap a kairos network.
		`,
				ArgsUsage: "Optionally takes a token rotation interval (seconds)",

				Action: func(c *cli.Context) error {
					l := int(^uint(0) >> 1)
					if c.Args().Present() {
						if i, err := strconv.Atoi(c.Args().Get(0)); err == nil {
							l = i
						}
					}
					cc := &providerConfig.Config{P2P: &providerConfig.P2P{NetworkToken: node.GenerateNewConnectionData(l).Base64()}}
					y, _ := yaml.Marshal(cc)
					fmt.Println(string(y))
					return nil
				},
			},
			{
				Name:      "generate-token",
				Aliases:   []string{"g"},
				UsageText: "Generate a network token",
				Usage:     "Creates a new token",
				Description: `
		Generates a new token which can be used to bootstrap a kairos network.
		`,
				ArgsUsage: "Optionally takes a token rotation interval (seconds)",

				Action: func(c *cli.Context) error {
					l := int(^uint(0) >> 1)
					if c.Args().Present() {
						if i, err := strconv.Atoi(c.Args().Get(0)); err == nil {
							l = i
						}
					}
					fmt.Println(node.GenerateNewConnectionData(l).Base64())
					return nil
				},
			},
		},
	}

	return app.Run(os.Args)
}
