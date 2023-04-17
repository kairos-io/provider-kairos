package cli

import (
	"encoding/base64"
	"fmt"
	edgeVPNClient "github.com/mudler/edgevpn/api/client"
	"github.com/mudler/edgevpn/api/client/service"
	"github.com/urfave/cli/v2"
	"strings"
)

var GetKubeConfigCMD = cli.Command{
	Name:      "get-kubeconfig",
	Usage:     "Return a deployment kubeconfig",
	UsageText: "Retrieve a kairos network kubeconfig (only for automated deployments)",
	Description: `
		Retrieve a network kubeconfig and prints out to screen.
		
		If a deployment was bootstrapped with a network token, you can use this command to retrieve the master node kubeconfig of a network id.
		
		For example:
		
		$ kairos get-kubeconfig --network-id kairos
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
}
