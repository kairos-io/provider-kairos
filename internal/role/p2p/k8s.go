package role

import (
	"errors"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

type K8sNode interface {
	PropagateData() error
	IP() string
	ClusterInit() bool
	HA() bool
	ProviderConfig() *providerConfig.Config
	SetRoleConfig(c *service.RoleConfig)
	RoleConfig() *service.RoleConfig
	GenerateEnv() map[string]string
	Service() (machine.Service, error)
	EnvUnit() string
	GenArgs() ([]string, error)
	DeployKubeVIP() error
	Token() (string, error)
	K8sBin() string
	SetupWorker(masterIP, nodeToken string) error
	Role() string
	WorkerArgs() ([]string, error)
	ServiceName() string
	Env() map[string]string
	Args() []string
	EnvFile() string
	SetRole(role string)
	SetIP(ip string)
	GuessInterface()
	Distro() string
}

func NewK8sNode(c *providerConfig.Config) (K8sNode, error) {
	switch {
	case c.K3s.Enabled:
		return &K3sNode{providerConfig: c, role: "master"}, nil
	case c.K3sAgent.Enabled:
		return &K3sNode{providerConfig: c, role: "worker"}, nil
	case c.K0s.Enabled:
		return &K0sNode{providerConfig: c, role: "master"}, nil
	case c.K0sWorker.Enabled:
		return &K0sNode{providerConfig: c, role: "worker"}, nil
	case utils.K3sBin() != "":
		return &K3sNode{providerConfig: c}, nil
	case utils.K0sBin() != "":
		return &K0sNode{providerConfig: c}, nil
	}

	return nil, errors.New("no k8s distro found")
}
