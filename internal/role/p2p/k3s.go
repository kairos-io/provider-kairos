package role

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	common "github.com/kairos-io/provider-kairos/v2/internal/role"
	service "github.com/mudler/edgevpn/api/client/service"
)

const (
	K3sDistroName = "k3s"
)

// K3sNode implements the base Node interface for K3s
type K3sNode struct {
	providerConfig *providerConfig.Config
	roleConfig     *service.RoleConfig
	ip             string
	iface          string
	ifaceIP        string
	role           string
}

// K3sControlPlane extends K3sNode with control plane functionality
type K3sControlPlane struct {
	*K3sNode
}

// K3sWorker extends K3sNode with worker functionality
type K3sWorker struct {
	*K3sNode
}

// Node interface implementation
func (k *K3sNode) GetIP() string {
	return k.ip
}

func (k *K3sNode) SetIP(ip string) {
	k.ip = ip
}

func (k *K3sNode) GetRole() string {
	if k.role == common.RoleWorker {
		return "agent"
	}

	return "server"
}

func (k *K3sNode) SetRole(role string) {
	k.role = role
}

func (k *K3sNode) GetDistro() string {
	return K3sDistroName
}

func (k *K3sNode) K8sBin() string {
	return utils.K3sBin()
}

func (k *K3sNode) GetConfig() *providerConfig.Config {
	return k.providerConfig
}

func (k *K3sNode) SetRoleConfig(c *service.RoleConfig) {
	k.roleConfig = c
}

func (k *K3sNode) GetRoleConfig() *service.RoleConfig {
	return k.roleConfig
}

func (k *K3sNode) GetService() (machine.Service, error) {
	if k.role == common.RoleWorker {
		return machine.K3sAgent()
	}
	return machine.K3s()
}

func (k *K3sNode) GetServiceName() string {
	if k.role == common.RoleWorker {
		return "k3s-agent"
	}
	return "k3s"
}

func (k *K3sNode) GetEnvFile() string {
	return machine.K3sEnvUnit(k.GetServiceName())
}

func (k *K3sNode) GenerateEnv() map[string]string {
	env := make(map[string]string)

	if k.role == common.RoleControlPlaneHA && k.role != common.RoleControlPlaneClusterInit {
		nodeToken, _ := k.GetToken()
		env["K3S_TOKEN"] = nodeToken
	}

	pConfig := k.GetConfig()

	if k.role == common.RoleWorker {
		if pConfig.K3sAgent.ReplaceEnv {
			env = pConfig.K3sAgent.Env
		} else {
			for k, v := range pConfig.K3sAgent.Env {
				env[k] = v
			}
		}
	} else {
		if pConfig.K3s.ReplaceEnv {
			env = pConfig.K3s.Env
		} else {
			for k, v := range pConfig.K3s.Env {
				env[k] = v
			}
		}
	}

	return env
}

func (k *K3sNode) GenerateArgs() ([]string, error) {
	if k.role == common.RoleWorker {
		return k.generateWorkerArgs()
	}
	return k.generateControlPlaneArgs()
}

func (k *K3sNode) GetToken() (string, error) {
	if k.role == common.RoleWorker {
		return k.GetRoleConfig().Client.Get("nodetoken", "token")
	}
	return k.GetRoleConfig().Client.Get("nodetoken", "token")
}

// ControlPlane interface implementation
func (k *K3sControlPlane) IsHA() bool {
	return k.role == common.RoleControlPlaneHA
}

func (k *K3sControlPlane) IsClusterInit() bool {
	return k.role == common.RoleControlPlaneClusterInit
}

func (k *K3sControlPlane) SetupHAToken() error {
	// K3s doesn't need a token for HA, it uses the node-token
	return nil
}

func (k *K3sControlPlane) DeployKubeVIP() error {
	pconfig := k.GetConfig()
	if !pconfig.KubeVIP.IsEnabled() {
		return nil
	}

	return deployKubeVIP(k.iface, k.ip, pconfig)
}

// Worker interface implementation
func (k *K3sWorker) SetupWorker(controlPlaneIP, nodeToken string) error {
	pconfig := k.GetConfig()

	nodeToken = strings.TrimRight(nodeToken, "\n")

	k3sConfig := providerConfig.K3s{}
	if pconfig.K3sAgent.Enabled {
		k3sConfig = pconfig.K3sAgent
	}

	env := map[string]string{
		"K3S_URL":   fmt.Sprintf("https://%s:6443", controlPlaneIP),
		"K3S_TOKEN": nodeToken,
	}

	if k3sConfig.ReplaceEnv {
		env = k3sConfig.Env
	} else {
		for k, v := range k3sConfig.Env {
			env[k] = v
		}
	}

	if err := utils.WriteEnv(machine.K3sEnvUnit("k3s-agent"),
		env,
	); err != nil {
		return err
	}

	return nil
}

// Helper methods
func (k *K3sNode) generateControlPlaneArgs() ([]string, error) {
	var args []string
	pconfig := k.GetConfig()

	if pconfig.P2P.UseVPNWithKubernetes() {
		args = append(args, "--flannel-iface=edgevpn0")
	}

	if pconfig.KubeVIP.IsEnabled() {
		args = append(args, fmt.Sprintf("--tls-san=%s", k.ip), fmt.Sprintf("--node-ip=%s", k.ifaceIP))
	}

	if pconfig.K3s.EmbeddedRegistry {
		args = append(args, "--embedded-registry")
	}

	if pconfig.P2P.Auto.HA.ExternalDB != "" {
		args = []string{fmt.Sprintf("--datastore-endpoint=%s", pconfig.P2P.Auto.HA.ExternalDB)}
	}

	if k.role == common.RoleControlPlaneHA && k.role != common.RoleControlPlaneClusterInit {
		clusterInitIP, _ := k.GetRoleConfig().Client.Get("control-plane", "ip")
		args = append(args, fmt.Sprintf("--server=https://%s:6443", clusterInitIP))
	}
	// The --cluster-init flag changes the embedded SQLite DB to etcd. We don't
	// want to do this if we're using an external DB.
	if k.role == common.RoleControlPlaneClusterInit && pconfig.P2P.Auto.HA.ExternalDB == "" {
		args = append(args, "--cluster-init")
	}

	if pconfig.K3s.ReplaceArgs {
		return pconfig.K3s.Args, nil
	}

	return append(args, pconfig.K3s.Args...), nil
}

func (k *K3sNode) generateWorkerArgs() ([]string, error) {
	pconfig := k.GetConfig()
	k3sConfig := providerConfig.K3s{}
	if pconfig.K3sAgent.Enabled {
		k3sConfig = pconfig.K3sAgent
	}

	args := []string{
		"--with-node-id",
	}

	if pconfig.P2P.UseVPNWithKubernetes() {
		ip := utils.GetInterfaceIP("edgevpn0")
		if ip == "" {
			return nil, errors.New("node doesn't have an ip yet")
		}
		args = append(args,
			fmt.Sprintf("--node-ip %s", ip),
			"--flannel-iface=edgevpn0")
	} else {
		iface := guessInterface(pconfig)
		ip := utils.GetInterfaceIP(iface)
		args = append(args,
			fmt.Sprintf("--node-ip %s", ip))
	}

	if k3sConfig.ReplaceArgs {
		return k3sConfig.Args, nil
	}

	return append(args, k3sConfig.Args...), nil
}

func (k *K3sNode) PropagateData() error {
	c := k.GetRoleConfig()
	tokenB, err := os.ReadFile("/var/lib/rancher/k3s/server/node-token")
	if err != nil {
		c.Logger.Error(err)
		return err
	}

	nodeToken := string(tokenB)
	nodeToken = strings.TrimRight(nodeToken, "\n")
	if nodeToken != "" {
		err := c.Client.Set("nodetoken", "token", nodeToken)
		if err != nil {
			c.Logger.Error(err)
		}
	}

	kubeB, err := os.ReadFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	kubeconfig := string(kubeB)
	if kubeconfig != "" {
		err := c.Client.Set("kubeconfig", "control-plane", base64.RawURLEncoding.EncodeToString(kubeB))
		if err != nil {
			c.Logger.Error(err)
		}
	}

	return nil
}

func (k *K3sNode) GuessInterface() {
	iface := guessInterface(k.GetConfig())
	ifaceIP := utils.GetInterfaceIP(iface)

	k.iface = iface
	k.ifaceIP = ifaceIP
}
