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

// configState holds the parsed configuration state.
type configState struct {
	k3sEnabled        bool
	k3sAgentEnabled   bool
	k0sEnabled        bool
	k0sWorkerEnabled  bool
	k3sDisabled       bool
	k3sAgentDisabled  bool
	k0sDisabled       bool
	k0sWorkerDisabled bool
	k3sBinAvailable   bool
	k0sBinAvailable   bool
	p2pConfigured     bool
	p2pAutoEnabled    bool
	p2pRoleSpecified  bool
}

// parseConfigState extracts and validates configuration state.
func parseConfigState(c *providerConfig.Config) (*configState, error) {
	state := &configState{
		// Check if any k8s distro is explicitly enabled
		k3sEnabled:       c.K3s.Enabled != nil && *c.K3s.Enabled,
		k3sAgentEnabled:  c.K3sAgent.Enabled != nil && *c.K3sAgent.Enabled,
		k0sEnabled:       c.K0s.Enabled != nil && *c.K0s.Enabled,
		k0sWorkerEnabled: c.K0sWorker.Enabled != nil && *c.K0sWorker.Enabled,

		// Check if any k8s distro is explicitly disabled
		k3sDisabled:       c.K3s.Enabled != nil && !*c.K3s.Enabled,
		k3sAgentDisabled:  c.K3sAgent.Enabled != nil && !*c.K3sAgent.Enabled,
		k0sDisabled:       c.K0s.Enabled != nil && !*c.K0s.Enabled,
		k0sWorkerDisabled: c.K0sWorker.Enabled != nil && !*c.K0sWorker.Enabled,

		// Check if binaries are available
		k3sBinAvailable: utils.K3sBin() != "",
		k0sBinAvailable: utils.K0sBin() != "",

		// Check if p2p is configured
		p2pConfigured:    c.P2P != nil,
		p2pAutoEnabled:   c.P2P != nil && c.P2P.Auto.Enable != nil && *c.P2P.Auto.Enable,
		p2pRoleSpecified: c.P2P != nil && c.P2P.Role != "",
	}

	// Validate conflicting configurations
	if state.p2pAutoEnabled && state.p2pRoleSpecified {
		return nil, errors.New("cannot specify both p2p.auto.enabled=true and p2p.role, they contradict each other")
	}

	return state, nil
}

// checkExplicitDisables validates if any k8s components are explicitly disabled.
func checkExplicitDisables(state *configState) error {
	if state.k3sDisabled || state.k3sAgentDisabled || state.k0sDisabled || state.k0sWorkerDisabled {
		if state.k3sDisabled {
			return errors.New("k3s is explicitly disabled")
		}
		if state.k3sAgentDisabled {
			return errors.New("k3s-agent is explicitly disabled")
		}
		if state.k0sDisabled {
			return errors.New("k0s is explicitly disabled")
		}
		if state.k0sWorkerDisabled {
			return errors.New("k0s-worker is explicitly disabled")
		}
	}
	return nil
}

// handleExplicitEnables handles cases where specific k8s components are explicitly enabled.
func handleExplicitEnables(c *providerConfig.Config, state *configState) (K8sNode, error) {
	if state.k3sEnabled {
		if !state.k3sBinAvailable {
			return nil, errors.New("k3s is enabled but k3s binary is not available")
		}
		return &K3sNode{providerConfig: c, role: RoleMaster}, nil
	}
	if state.k3sAgentEnabled {
		if !state.k3sBinAvailable {
			return nil, errors.New("k3s-agent is enabled but k3s binary is not available")
		}
		return &K3sNode{providerConfig: c, role: RoleWorker}, nil
	}
	if state.k0sEnabled {
		if !state.k0sBinAvailable {
			return nil, errors.New("k0s is enabled but k0s binary is not available")
		}
		return &K0sNode{providerConfig: c, role: RoleMaster}, nil
	}
	if state.k0sWorkerEnabled {
		if !state.k0sBinAvailable {
			return nil, errors.New("k0s-worker is enabled but k0s binary is not available")
		}
		return &K0sNode{providerConfig: c, role: RoleWorker}, nil
	}
	return nil, nil
}

// handleP2PRole handles cases where P2P is configured with a specific role.
func handleP2PRole(c *providerConfig.Config, state *configState) (K8sNode, error) {
	if !state.p2pRoleSpecified {
		return nil, nil
	}

	switch c.P2P.Role {
	case RoleMaster:
		if state.k3sBinAvailable {
			return &K3sNode{providerConfig: c, role: RoleMaster}, nil
		}
		if state.k0sBinAvailable {
			return &K0sNode{providerConfig: c, role: RoleMaster}, nil
		}
		return nil, errors.New("p2p.role=master specified but no k8s binary (k3s or k0s) is available")
	case RoleWorker:
		if state.k3sBinAvailable {
			return &K3sNode{providerConfig: c, role: RoleWorker}, nil
		}
		if state.k0sBinAvailable {
			return &K0sNode{providerConfig: c, role: RoleWorker}, nil
		}
		return nil, errors.New("p2p.role=worker specified but no k8s binary (k3s or k0s) is available")
	default:
		return nil, errors.New("invalid p2p.role specified, must be 'master' or 'worker'")
	}
}

// handleP2PAuto handles cases where P2P is configured with auto mode.
func handleP2PAuto(c *providerConfig.Config, state *configState) (K8sNode, error) {
	if !state.p2pAutoEnabled {
		return nil, nil
	}

	if state.k3sBinAvailable {
		return &K3sNode{providerConfig: c}, nil // No role set, will be assigned automatically
	}
	if state.k0sBinAvailable {
		return &K0sNode{providerConfig: c}, nil // No role set, will be assigned automatically
	}
	return nil, errors.New("p2p.auto.enabled=true but no k8s binary (k3s or k0s) is available")
}

// handleP2PConfigured handles cases where P2P is configured but no role or auto mode specified.
func handleP2PConfigured(state *configState) error {
	if !state.p2pConfigured {
		return nil
	}
	return errors.New("p2p is configured but no k8s component is explicitly enabled. To enable k8s with p2p, either: 1) explicitly enable k3s, k3s-agent, k0s, or k0s-worker, or 2) set p2p.auto.enabled=true")
}

func NewK8sNode(c *providerConfig.Config) (K8sNode, error) {
	// Parse and validate configuration state
	state, err := parseConfigState(c)
	if err != nil {
		return nil, err
	}

	// Check if user explicitly disabled k8s
	if err := checkExplicitDisables(state); err != nil {
		return nil, err
	}

	// Case 1: User explicitly enabled specific k8s components
	if node, err := handleExplicitEnables(c, state); err != nil || node != nil {
		return node, err
	}

	// Case 2: P2P is configured with specific role
	if node, err := handleP2PRole(c, state); err != nil || node != nil {
		return node, err
	}

	// Case 3: P2P is configured with auto mode (no role specified)
	if node, err := handleP2PAuto(c, state); err != nil || node != nil {
		return node, err
	}

	// Case 4: P2P is configured but no role or auto mode specified
	if err := handleP2PConfigured(state); err != nil {
		return nil, err
	}

	// Case 5: No k8s configuration found
	return nil, errors.New("no k8s configuration found. To enable k8s, either: 1) explicitly enable k3s, k3s-agent, k0s, or k0s-worker, or 2) configure p2p")
}
