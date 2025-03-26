package role

import (
	"errors"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	common "github.com/kairos-io/provider-kairos/v2/internal/role"
	service "github.com/mudler/edgevpn/api/client/service"
)

// Node represents any kubernetes node, regardless of its role
type Node interface {
	// Core node functionality
	GetIP() string
	SetIP(string)
	GetRole() string
	SetRole(string)
	GetDistro() string // k3s, k0s
	K8sBin() string    // Returns the path to the Kubernetes binary

	// Configuration
	GetConfig() *providerConfig.Config
	SetRoleConfig(*service.RoleConfig)
	GetRoleConfig() *service.RoleConfig

	// Service management
	GetService() (machine.Service, error)
	GetServiceName() string
	GetEnvFile() string
	GenerateEnv() map[string]string

	// Node operations
	GenerateArgs() ([]string, error)
	PropagateData() error
	GetToken() (string, error)
}

// ControlPlaneNode represents additional functionality specific to control plane nodes
type ControlPlaneNode interface {
	Node
	IsHA() bool
	IsClusterInit() bool
	SetupHAToken() error
	DeployKubeVIP() error
}

// WorkerNode represents additional functionality specific to worker nodes
type WorkerNode interface {
	Node
	SetupWorker(controlPlaneIP, nodeToken string) error
}

func NewNode(config *providerConfig.Config, role string) (Node, error) {
	switch {
	case config.K3s.Enabled:
		return &K3sControlPlane{K3sNode: &K3sNode{providerConfig: config}}, nil
	case config.K3sAgent.Enabled:
		return &K3sWorker{K3sNode: &K3sNode{providerConfig: config}}, nil
	case config.K0s.Enabled:
		return &K0sControlPlane{K0sNode: &K0sNode{providerConfig: config}}, nil
	case config.K0sWorker.Enabled:
		return &K0sWorker{K0sNode: &K0sNode{providerConfig: config}}, nil
	case role == common.RoleWorker:
		if utils.K3sBin() != "" {
			return &K3sWorker{K3sNode: &K3sNode{providerConfig: config}}, nil
		}
		if utils.K0sBin() != "" {
			return &K0sWorker{K0sNode: &K0sNode{providerConfig: config}}, nil
		}
	default:
		if utils.K3sBin() != "" {
			return &K3sControlPlane{K3sNode: &K3sNode{providerConfig: config}}, nil
		}
		if utils.K0sBin() != "" {
			return &K0sControlPlane{K0sNode: &K0sNode{providerConfig: config}}, nil
		}
	}

	return nil, errors.New("no k8s distro found")
}

// Helper function to convert Node to ControlPlaneNode
func AsControlPlane(n Node) (ControlPlaneNode, bool) {
	cp, ok := n.(ControlPlaneNode)
	return cp, ok
}

// Helper function to convert Node to WorkerNode
func AsWorker(n Node) (WorkerNode, bool) {
	w, ok := n.(WorkerNode)
	return w, ok
}
