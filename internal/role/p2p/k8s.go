package role

import (
	"errors"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

type ServiceDefinition interface {
	ServiceName() string
	Role() string
	Env() map[string]string
	Args() ([]string, error)
	K8sBin() string
	EnvFile() string
}

type K8sControlPlane interface {
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
	Args() ([]string, error)
	DeployKubeVIP() error
	Token() (string, error)
	K8sBin() string
	Role() string
	ServiceName() string
	Env() map[string]string
	EnvFile() string
	SetRole(role string)
	SetIP(ip string)
	GuessInterface()
	Distro() string
	SetupHAToken() error
}

type K8sWorker interface {
	IP() string
	ProviderConfig() *providerConfig.Config
	SetRoleConfig(c *service.RoleConfig)
	RoleConfig() *service.RoleConfig
	Service() (machine.Service, error)
	Token() (string, error)
	K8sBin() string
	SetupWorker(controlPlaneIP, nodeToken string) error
	Role() string
	Args() ([]string, error)
	ServiceName() string
	Env() map[string]string
	EnvFile() string
	SetRole(role string)
	SetIP(ip string)
	GuessInterface()
	Distro() string
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
