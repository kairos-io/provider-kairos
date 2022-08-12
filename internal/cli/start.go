package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	edgeVPNClient "github.com/mudler/edgevpn/api/client"

	providerConfig "github.com/c3os-io/provider-c3os/internal/provider/config"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v1"

	"github.com/mudler/edgevpn/api/client/service"
	"github.com/mudler/edgevpn/pkg/node"
)

var networkAPI = []cli.Flag{
	&cli.StringFlag{
		Name:  "api",
		Usage: "API Address",
		Value: "http://localhost:8080",
	},
	&cli.StringFlag{
		Name:  "network-id",
		Value: "c3os",
		Usage: "Kubernetes Network Deployment ID",
	},
}

const recoveryAddr = "127.0.0.1:2222"

func Start() error {
	app := &cli.App{
		Name:    "c3os",
		Version: "0.1",
		Author:  "Ettore Di Giacinto",
		Usage:   "c3os CLI to bootstrap, upgrade, connect and manage a c3os network",
		Description: `
The c3os CLI can be used to manage a c3os box and perform all day-two tasks, like:
- register a node
- connect to a node in recovery mode
- to establish a VPN connection
- set, list roles
- interact with the network API

and much more.

For all the example cases, see: https://docs.c3os.io .
`,
		UsageText: ``,
		Copyright: "Ettore Di Giacinto",
		Commands: []cli.Command{
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
						Name:   "token",
						EnvVar: "TOKEN",
					},
					&cli.StringFlag{
						Name:   "service",
						EnvVar: "SERVICE",
					},
					&cli.StringFlag{
						Name:   "password",
						EnvVar: "PASSWORD",
					},
					&cli.StringFlag{
						Name:   "listen",
						EnvVar: "LISTEN",
						Value:  recoveryAddr,
					},
				},
				Action: func(c *cli.Context) error {
					return StartRecoveryService(c.String("token"), c.String("service"), c.String("password"), c.String("listen"))
				},
			},
			{
				Name:      "register",
				UsageText: "register --reboot --device /dev/sda /image/snapshot.png",
				Usage:     "Registers and bootstraps a node",
				Description: `
		Bootstraps a node which is started in pairing mode. It can send over a configuration file used to install the c3os node.
		
		For example:
		$ c3os register --config config.yaml --device /dev/sda ~/Downloads/screenshot.png
		
		will decode the QR code from ~/Downloads/screenshot.png and bootstrap the node remotely.
		
		If the image is omitted, a screenshot will be taken and used to decode the QR code.
		
		See also https://docs.c3os.io/installation/device_pairing/ for documentation.
		`,
				ArgsUsage: "Register optionally accepts an image. If nothing is passed will take a screenshot of the screen and try to decode the QR code",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "C3OS YAML configuration file",
					},
					&cli.StringFlag{
						Name:  "device",
						Usage: "Device used for the installation target",
					},
					&cli.BoolFlag{
						Name:  "reboot",
						Usage: "Reboot node after installation",
					},
					&cli.BoolFlag{
						Name:  "poweroff",
						Usage: "Shutdown node after installation",
					},
					&cli.StringFlag{
						Name:  "log-level",
						Usage: "Set log level",
					},
				},

				Action: func(c *cli.Context) error {
					args := c.Args()
					var ref string
					if len(args) == 1 {
						ref = args[0]
					}

					return register(c.String("log-level"), ref, c.String("config"), c.String("device"), c.Bool("reboot"), c.Bool("poweroff"))
				},
			},
			{
				Name:      "bridge",
				UsageText: "bridge --network-token XXX",
				Usage:     "Connect to a c3os VPN network",
				Description: `
		Starts a bridge with a c3os network or a node. 
		
		# With a network
		
		By default, "bridge" will create a VPN network connection to the node with the token supplied, thus it requires elevated permissions in order to work.
		
		For example:
		
		$ sudo c3os bridge --network-token <TOKEN>
		
		Will start a VPN, which local ip is fixed to 10.1.0.254 (tweakable with --address).
		
		The API will also be accessible at http://127.0.0.1:8080
		
		# With a node
		
		"c3os bridge" can be used also to connect over to a node in recovery mode. When operating in this modality c3os bridge requires no specific permissions, indeed a tunnel
		will be created locally to connect to the machine remotely.
		
		For example:
		
		$ c3os bridge --qr-code-image /path/to/image.png
		
		Will scan the QR code in the image and connect over. Further instructions on how to connect over will be printed out to the screen.
		
		See also: https://docs.c3os.io/after_install/troubleshooting/#connect-to-the-cluster-network and https://docs.c3os.io/after_install/recovery_mode/
		
		`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "network-token",
						Required: false,
						EnvVar:   "NETWORK_TOKEN",
						Usage:    "Network token to connect over",
					},
					&cli.StringFlag{
						Name:     "log-level",
						Required: false,
						EnvVar:   "LOGLEVEL",
						Value:    "info",
						Usage:    "Bridge log level",
					},
					&cli.BoolFlag{
						Name:     "qr-code-snapshot",
						Required: false,
						Usage:    "Bool to take a local snapshot instead of reading from an image file for recovery",
						EnvVar:   "QR_CODE_SNAPSHOT",
					},
					&cli.StringFlag{
						Name:     "qr-code-image",
						Usage:    "Path to an image containing a valid QR code for recovery mode",
						Required: false,
						EnvVar:   "QR_CODE_IMAGE",
					},
					&cli.StringFlag{
						Name:  "api",
						Value: "127.0.0.1:8080",
						Usage: "Listening API url",
					},
					&cli.BoolFlag{
						Name:   "dhcp",
						EnvVar: "DHCP",
						Usage:  "Enable DHCP",
					},
					&cli.StringFlag{
						Value:  "10.1.0.254/24",
						Name:   "address",
						EnvVar: "ADDRESS",
						Usage:  "Specify an address for the bridge",
					},
					&cli.StringFlag{
						Value:  "/tmp/c3os",
						Name:   "lease-dir",
						EnvVar: "lease-dir",
						Usage:  "DHCP Lease directory",
					},
				},
				Action: bridge,
			},
			{
				Name:      "get-kubeconfig",
				Usage:     "Return a deployment kubeconfig",
				UsageText: "Retrieve a c3os network kubeconfig (only for automated deployments)",
				Description: `
		Retrieve a network kubeconfig and prints out to screen.
		
		If a deployment was bootstrapped with a network token, you can use this command to retrieve the master node kubeconfig of a network id.
		
		For example:
		
		$ c3os get-kubeconfig --network-id c3os
		`,
				Flags: networkAPI,
				Action: func(c *cli.Context) error {
					cc := service.NewClient(
						c.String("network-id"),
						edgeVPNClient.NewClient(edgeVPNClient.WithHost(c.String("api"))))
					str, _ := cc.Get("kubeconfig", "master")
					b, _ := base64.RawURLEncoding.DecodeString(str)
					masterIP, _ := cc.Get("master", "ip")
					fmt.Println(strings.ReplaceAll(string(b), "127.0.0.1", masterIP))
					return nil
				},
			},
			{
				Name:  "role",
				Usage: "Set or list node roles",
				Subcommands: []cli.Command{
					{
						Flags:     networkAPI,
						Name:      "set",
						Usage:     "Set a node role",
						UsageText: "c3os role set <UUID> master",
						Description: `
		Sets a node role propagating the setting to the network.
		
		A role must be set prior to the node joining a network. You can retrieve a node UUID by running "c3os uuid".
		
		Example:
		
		$ (node A) c3os uuid
		$ (node B) c3os role set <UUID of node A> master
		`,
						Action: func(c *cli.Context) error {
							cc := service.NewClient(
								c.String("network-id"),
								edgeVPNClient.NewClient(edgeVPNClient.WithHost(c.String("api"))))
							return cc.Set("role", c.Args()[0], c.Args()[1])
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
			},
			{
				Name:      "create-config",
				Aliases:   []string{"c"},
				UsageText: "Create a config with a generated network token",

				Usage: "Creates a pristine config file",
				Description: `
		Prints a vanilla YAML configuration on screen which can be used to bootstrap a c3os network.
		`,
				ArgsUsage: "Optionally takes a token rotation interval (seconds)",

				Action: func(c *cli.Context) error {
					l := int(^uint(0) >> 1)
					args := c.Args()
					if len(args) > 0 {
						if i, err := strconv.Atoi(args[0]); err == nil {
							l = i
						}
					}
					cc := &providerConfig.Config{C3OS: &providerConfig.C3OS{NetworkToken: node.GenerateNewConnectionData(l).Base64()}}
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
		Generates a new token which can be used to bootstrap a c3os network.
		`,
				ArgsUsage: "Optionally takes a token rotation interval (seconds)",

				Action: func(c *cli.Context) error {
					l := int(^uint(0) >> 1)
					args := c.Args()
					if len(args) > 0 {
						if i, err := strconv.Atoi(args[0]); err == nil {
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
