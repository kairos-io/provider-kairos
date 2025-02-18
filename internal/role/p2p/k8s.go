package role

import (
	"github.com/kairos-io/kairos-sdk/machine"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

type K8sNode interface {
	propagateData() error
	IP() string
	ClusterInit() bool
	HA() bool
	SetProviderConfig(c *providerConfig.Config)
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
	CmdFirstArg() string
	WorkerArgs() ([]string, error)
}
