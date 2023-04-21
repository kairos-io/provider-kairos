package cli

import (
	"fmt"

	edgeVPNClient "github.com/mudler/edgevpn/api/client"
	"github.com/mudler/edgevpn/api/client/service"
	"github.com/urfave/cli/v2"
)

var RoleCMD = cli.Command{
	Name:  "role",
	Usage: "Set or list node roles",
	Subcommands: []*cli.Command{
		{
			Flags:     networkAPI,
			Name:      "set",
			Usage:     "Set a node role",
			UsageText: "kairos role set <UUID> master",
			Description: `
		Sets a node role propagating the setting to the network.

		A role must be set prior to the node joining a network. You can retrieve a node UUID by running "kairos uuid".

		Example:

		$ (node A) kairos uuid
		$ (node B) kairos role set <UUID of node A> master
		`,
			Action: func(c *cli.Context) error {
				cc := service.NewClient(
					c.String("network-id"),
					edgeVPNClient.NewClient(edgeVPNClient.WithHost(c.String("api"))))
				return cc.Set("role", c.Args().Get(0), c.Args().Get(1))
			},
		},
		{
			Flags:       networkAPI,
			Name:        "list",
			Description: "List node roles",
			Action: func(c *cli.Context) error {
				cc := service.NewClient(
					c.String("network-id"),
					edgeVPNClient.NewClient(edgeVPNClient.WithHost(c.String("api"))))
				advertizing, _ := cc.AdvertizingNodes()
				fmt.Println("Node\tRole")
				for _, a := range advertizing {
					role, _ := cc.Get("role", a)
					fmt.Printf("%s\t%s\n", a, role)
				}
				return nil
			},
		},
	},
}
