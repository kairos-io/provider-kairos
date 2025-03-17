package cli

import (
	"encoding/base64"
	"fmt"
	"strings"

	edgeVPNClient "github.com/mudler/edgevpn/api/client"
	"github.com/mudler/edgevpn/api/client/service"
	"github.com/urfave/cli/v2"
)

var GetKubeConfigCMD = cli.Command{
	Name:      "get-kubeconfig",
	Usage:     "Return a deployment kubeconfig",
	UsageText: "Retrieve a kairos network kubeconfig (only for automated deployments)",
	Description: `
		Retrieve a network kubeconfig and prints out to screen.
		
		If a deployment was bootstrapped with a network token, you can use this command to retrieve the control-plane node kubeconfig of a network id.
		
		For example:
		
		$ kairos get-kubeconfig --network-id kairos
		`,
	Flags: networkAPI,
	Action: func(c *cli.Context) error {
		cc := service.NewClient(
			c.String("network-id"),
			edgeVPNClient.NewClient(edgeVPNClient.WithHost(c.String("api"))))
		str, _ := cc.Get("kubeconfig", "control-plane")
		b, _ := base64.RawURLEncoding.DecodeString(str)
		masterIP, _ := cc.Get("control-plane", "ip")
		fmt.Println(strings.ReplaceAll(string(b), "127.0.0.1", masterIP))
		return nil
	},
}
