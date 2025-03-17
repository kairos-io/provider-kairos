package role

import (
	"fmt"
	"net"

	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
)

const (
	RoleWorker                  = "worker"
	RoleControlPlane            = "master"
	RoleControlPlaneHA          = "master/ha"
	RoleControlPlaneClusterInit = "master/clusterinit"
	RoleAuto                    = "auto"
)

func guessInterface(pconfig *providerConfig.Config) string {
	if pconfig.KubeVIP.Interface != "" {
		return pconfig.KubeVIP.Interface
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("failed getting system interfaces")
		return ""
	}
	for _, i := range ifaces {
		if i.Name != "lo" {
			return i.Name
		}
	}
	return ""
}
