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
	// Check if any k8s distro is explicitly enabled
	k3sEnabled := c.K3s.Enabled != nil && *c.K3s.Enabled
	k3sAgentEnabled := c.K3sAgent.Enabled != nil && *c.K3sAgent.Enabled
	k0sEnabled := c.K0s.Enabled != nil && *c.K0s.Enabled
	k0sWorkerEnabled := c.K0sWorker.Enabled != nil && *c.K0sWorker.Enabled

	// Check if any k8s distro is explicitly disabled
	k3sDisabled := c.K3s.Enabled != nil && !*c.K3s.Enabled
	k3sAgentDisabled := c.K3sAgent.Enabled != nil && !*c.K3sAgent.Enabled
	k0sDisabled := c.K0s.Enabled != nil && !*c.K0s.Enabled
	k0sWorkerDisabled := c.K0sWorker.Enabled != nil && !*c.K0sWorker.Enabled

	// Check if binaries are available
	k3sBinAvailable := utils.K3sBin() != ""
	k0sBinAvailable := utils.K0sBin() != ""

	// Check if p2p is configured
	p2pConfigured := c.P2P != nil
	p2pAutoEnabled := p2pConfigured && c.P2P.Auto.Enable != nil && *c.P2P.Auto.Enable
	p2pRoleSpecified := p2pConfigured && c.P2P.Role != ""

	// Validate conflicting configurations
	if p2pAutoEnabled && p2pRoleSpecified {
		return nil, errors.New("cannot specify both p2p.auto.enabled=true and p2p.role, they contradict each other")
	}

	// Check if user explicitly disabled k8s
	if k3sDisabled || k3sAgentDisabled || k0sDisabled || k0sWorkerDisabled {
		if k3sDisabled {
			return nil, errors.New("k3s is explicitly disabled")
		}
		if k3sAgentDisabled {
			return nil, errors.New("k3s-agent is explicitly disabled")
		}
		if k0sDisabled {
			return nil, errors.New("k0s is explicitly disabled")
		}
		if k0sWorkerDisabled {
			return nil, errors.New("k0s-worker is explicitly disabled")
		}
	}

	// Case 1: User explicitly enabled specific k8s components
	if k3sEnabled {
		if !k3sBinAvailable {
			return nil, errors.New("k3s is enabled but k3s binary is not available")
		}
		return &K3sNode{providerConfig: c, role: RoleMaster}, nil
	}
	if k3sAgentEnabled {
		if !k3sBinAvailable {
			return nil, errors.New("k3s-agent is enabled but k3s binary is not available")
		}
		return &K3sNode{providerConfig: c, role: RoleWorker}, nil
	}
	if k0sEnabled {
		if !k0sBinAvailable {
			return nil, errors.New("k0s is enabled but k0s binary is not available")
		}
		return &K0sNode{providerConfig: c, role: RoleMaster}, nil
	}
	if k0sWorkerEnabled {
		if !k0sBinAvailable {
			return nil, errors.New("k0s-worker is enabled but k0s binary is not available")
		}
		return &K0sNode{providerConfig: c, role: RoleWorker}, nil
	}

	// Case 2: P2P is configured with specific role
	if p2pRoleSpecified {
		switch c.P2P.Role {
		case RoleMaster:
			if k3sBinAvailable {
				return &K3sNode{providerConfig: c, role: RoleMaster}, nil
			}
			if k0sBinAvailable {
				return &K0sNode{providerConfig: c, role: RoleMaster}, nil
			}
			return nil, errors.New("p2p.role=master specified but no k8s binary (k3s or k0s) is available")
		case RoleWorker:
			if k3sBinAvailable {
				return &K3sNode{providerConfig: c, role: RoleWorker}, nil
			}
			if k0sBinAvailable {
				return &K0sNode{providerConfig: c, role: RoleWorker}, nil
			}
			return nil, errors.New("p2p.role=worker specified but no k8s binary (k3s or k0s) is available")
		default:
			return nil, errors.New("invalid p2p.role specified, must be 'master' or 'worker'")
		}
	}

	// Case 3: P2P is configured with auto mode (no role specified)
	if p2pAutoEnabled {
		if k3sBinAvailable {
			return &K3sNode{providerConfig: c}, nil // No role set, will be assigned automatically
		}
		if k0sBinAvailable {
			return &K0sNode{providerConfig: c}, nil // No role set, will be assigned automatically
		}
		return nil, errors.New("p2p.auto.enabled=true but no k8s binary (k3s or k0s) is available")
	}

	// Case 4: P2P is configured but no role or auto mode specified
	// In this case, we need explicit k8s enablement to proceed
	if p2pConfigured {
		return nil, errors.New("p2p is configured but no k8s component is explicitly enabled. To enable k8s with p2p, either: 1) explicitly enable k3s, k3s-agent, k0s, or k0s-worker, or 2) set p2p.auto.enabled=true")
	}

	// Case 5: No k8s configuration found
	return nil, errors.New("no k8s configuration found. To enable k8s, either: 1) explicitly enable k3s, k3s-agent, k0s, or k0s-worker, or 2) configure p2p")
}
