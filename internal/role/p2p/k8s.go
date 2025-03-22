package role

import (
	"errors"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

type ServiceDefinition interface {
	Args() ([]string, error)
	Env() map[string]string
	EnvFile() string
	K8sBin() string
	Role() string
	ServiceName() string
}

type K8sControlPlane interface {
	Args() ([]string, error)
	ClusterInit() bool
	DeployKubeVIP() error
	Distro() string
	EnvUnit() string
	GenerateEnv() map[string]string
	GuessInterface()
	HA() bool
	IP() string
	K8sBin() string
	PropagateData() error
	ProviderConfig() *providerConfig.Config
	Role() string
	RoleConfig() *service.RoleConfig
	Service() (machine.Service, error)
	SetIP(ip string)
	SetRole(role string)
	SetRoleConfig(c *service.RoleConfig)
	SetupHAToken() error
	Token() (string, error)
}

type K8sWorker interface {
	Args() ([]string, error)
	Distro() string
	IP() string
	K8sBin() string
	ProviderConfig() *providerConfig.Config
	Role() string
	RoleConfig() *service.RoleConfig
	Service() (machine.Service, error)
	SetIP(ip string)
	SetRole(role string)
	SetRoleConfig(c *service.RoleConfig)
	SetupWorker(controlPlaneIP, nodeToken string) error
	Token() (string, error)
}

func NewServiceDefinition(c *providerConfig.Config) (ServiceDefinition, error) {
	switch {
	case c.K3s.Enabled:
		return &K3sControlPlane{providerConfig: c}, nil
	case c.K0s.Enabled:
		return &K0sControlPlane{providerConfig: c}, nil
	case c.K3sAgent.Enabled:
		return &K3sWorker{providerConfig: c}, nil
	case c.K0sWorker.Enabled:
		return &K0sWorker{providerConfig: c}, nil
	}

	return nil, errors.New("no k8s distro found")
}

func NewK8sControlPlane(c *providerConfig.Config) (K8sControlPlane, error) {
	switch {
	case c.K3s.Enabled:
		return &K3sControlPlane{providerConfig: c, role: "control-plane"}, nil
	case c.K0s.Enabled:
		return &K0sControlPlane{providerConfig: c, role: "control-plane"}, nil
	case utils.K3sBin() != "":
		return &K3sControlPlane{providerConfig: c}, nil
	case utils.K0sBin() != "":
		return &K0sControlPlane{providerConfig: c}, nil
	}

	return nil, errors.New("no k8s distro found")
}

func NewK8sWorker(c *providerConfig.Config) (K8sWorker, error) {
	switch {
	case c.K3sAgent.Enabled:
		return &K3sWorker{providerConfig: c, role: "worker"}, nil
	case c.K0sWorker.Enabled:
		return &K0sWorker{providerConfig: c, role: "worker"}, nil
	case utils.K3sBin() != "":
		return &K3sWorker{providerConfig: c}, nil
	case utils.K0sBin() != "":
		return &K0sWorker{providerConfig: c}, nil
	}

	return nil, errors.New("no k8s distro found")
}
