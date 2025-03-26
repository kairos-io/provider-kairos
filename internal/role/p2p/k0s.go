package role

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	common "github.com/kairos-io/provider-kairos/v2/internal/role"
	service "github.com/mudler/edgevpn/api/client/service"
	"gopkg.in/yaml.v2"
)

const (
	K0sDistroName = "k0s"
)

// K0sNode implements the base Node interface for K0s
type K0sNode struct {
	providerConfig *providerConfig.Config
	roleConfig     *service.RoleConfig
	ip             string
	role           string
}

// K0sControlPlane extends K0sNode with control plane functionality
type K0sControlPlane struct {
	*K0sNode
}

// K0sWorker extends K0sNode with worker functionality
type K0sWorker struct {
	*K0sNode
}

// Node interface implementation
func (k *K0sNode) GetIP() string {
	return k.ip
}

func (k *K0sNode) SetIP(ip string) {
	k.ip = ip
}

func (k *K0sNode) GetRole() string {
	if k.role == common.RoleWorker {
		return "worker"
	}

	return "controller"
}

func (k *K0sNode) SetRole(role string) {
	k.role = role
}

func (k *K0sNode) GetDistro() string {
	return K0sDistroName
}

func (k *K0sNode) K8sBin() string {
	return utils.K0sBin()
}

func (k *K0sNode) GetConfig() *providerConfig.Config {
	return k.providerConfig
}

func (k *K0sNode) SetRoleConfig(c *service.RoleConfig) {
	k.roleConfig = c
}

func (k *K0sNode) GetRoleConfig() *service.RoleConfig {
	return k.roleConfig
}

func (k *K0sNode) GetService() (machine.Service, error) {
	if k.role == common.RoleWorker {
		return machine.K0sWorker()
	}
	return machine.K0s()
}

func (k *K0sNode) GetServiceName() string {
	if k.role == common.RoleWorker {
		return "k0sworker"
	}
	return "k0scontroller"
}

func (k *K0sNode) GetEnvFile() string {
	return machine.K0sEnvUnit(k.GetServiceName())
}

func (k *K0sNode) GenerateEnv() map[string]string {
	env := make(map[string]string)

	if k.role == common.RoleControlPlaneHA && k.role != common.RoleControlPlaneClusterInit {
		nodeToken, _ := k.GetToken()
		env["K0S_TOKEN"] = nodeToken
	}

	pConfig := k.GetConfig()

	if k.role == common.RoleWorker {
		if pConfig.K0sWorker.ReplaceEnv {
			env = pConfig.K0sWorker.Env
		} else {
			for k, v := range pConfig.K0sWorker.Env {
				env[k] = v
			}
		}
	} else {
		if pConfig.K0s.ReplaceEnv {
			env = pConfig.K0s.Env
		} else {
			for k, v := range pConfig.K0s.Env {
				env[k] = v
			}
		}
	}

	return env
}

func (k *K0sNode) GenerateArgs() ([]string, error) {
	if k.role == common.RoleWorker {
		return k.generateWorkerArgs()
	}
	return k.generateControlPlaneArgs()
}

func (k *K0sNode) GetToken() (string, error) {
	if k.role == common.RoleWorker {
		return k.GetRoleConfig().Client.Get("workertoken", "token")
	}
	return k.GetRoleConfig().Client.Get("controllertoken", "token")
}

// ControlPlaneNode interface implementation
func (k *K0sControlPlane) IsHA() bool {
	return k.role == common.RoleControlPlaneHA
}

func (k *K0sControlPlane) IsClusterInit() bool {
	return k.role == common.RoleControlPlaneClusterInit
}

func (k *K0sControlPlane) SetupHAToken() error {
	controlPlaneToken, err := k.GetToken()
	if err != nil {
		return err
	}

	if controlPlaneToken == "" {
		return errors.New("control plane token is not there")
	}

	if err := os.WriteFile("/etc/k0s/token", []byte(controlPlaneToken), 0644); err != nil {
		return err
	}

	return nil
}

func (k *K0sControlPlane) DeployKubeVIP() error {
	pconfig := k.GetConfig()
	if pconfig.KubeVIP.IsEnabled() {
		return errors.New("KubeVIP is not yet supported with k0s")
	}

	return nil
}

// WorkerNode interface implementation
func (k *K0sWorker) SetupWorker(_, nodeToken string) error {
	if err := os.WriteFile("/etc/k0s/token", []byte(nodeToken), 0644); err != nil {
		return err
	}

	return nil
}

// Helper methods
func (k *K0sNode) generateControlPlaneArgs() ([]string, error) {
	var args []string

	if k.role == common.RoleControlPlaneSingle {
		args = append(args, "--single")
	}

	// Generate a new k0s config
	_, err := utils.SH("k0s config create > /etc/k0s/k0s.yaml")
	if err != nil {
		return args, err
	}
	args = append(args, "--config /etc/k0s/k0s.yaml")

	data, err := os.ReadFile("/etc/k0s/k0s.yaml")
	if err != nil {
		return nil, err
	}

	var k0sConfig map[any]any
	err = yaml.Unmarshal(data, &k0sConfig)
	if err != nil {
		return args, err
	}

	// check if the k0s config has an api address
	spec, ok := k0sConfig["spec"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have a spec")
	}
	api, ok := spec["api"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have an api")
	}
	// by default k0s uses the first IP address of the machine as the api address, but we want to use the edgevpn IP
	api["address"] = k.GetIP()

	spec["api"] = api

	network, ok := spec["network"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have a network")
	}
	kubeRouter, ok := network["kuberouter"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have a kuberouter")
	}

	// by default k0s uses the port 8080 for the metrics but this conflicts with the edgevpn API port
	kubeRouter["metricsPort"] = 9090
	network["kuberouter"] = kubeRouter
	spec["network"] = network

	storage, ok := spec["storage"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have a storage")
	}
	etcd, ok := storage["etcd"].(map[any]any)
	if !ok {
		return args, errors.New("k0s config does not have a etcd")
	}
	// just like the api address, we want to use the edgevpn IP for the etcd peer address
	etcd["peerAddress"] = k.GetIP()

	storage["etcd"] = etcd
	spec["storage"] = storage

	k0sConfig["spec"] = spec

	// write the k0s config back to the file
	data, err = yaml.Marshal(k0sConfig)
	if err != nil {
		return args, err
	}
	err = os.WriteFile("/etc/k0s/k0s.yaml", data, 0644)
	if err != nil {
		return args, err
	}

	pconfig := k.GetConfig()
	if !pconfig.P2P.UseVPNWithKubernetes() {
		return args, errors.New("Having a VPN but not using it for Kubernetes is not yet supported with k0s")
	}

	if pconfig.KubeVIP.IsEnabled() {
		return args, errors.New("KubeVIP is not yet supported with k0s")
	}

	if pconfig.P2P.Auto.HA.ExternalDB != "" {
		return args, errors.New("ExternalDB is not yet supported with k0s")
	}

	if k.role == common.RoleControlPlaneHA {
		args = append(args, "--token-file /etc/k0s/token")
	}

	return args, nil
}

func (k *K0sNode) generateWorkerArgs() ([]string, error) {
	pconfig := k.GetConfig()
	k0sConfig := pconfig.K0sWorker
	args := []string{"--token-file /etc/k0s/token"}

	if k0sConfig.ReplaceArgs {
		return k0sConfig.Args, nil
	}

	return append(args, k0sConfig.Args...), nil
}

func (k *K0sNode) PropagateData() error {
	c := k.GetRoleConfig()
	controllerToken, err := utils.SH("k0s token create --role=controller") //nolint:errcheck
	if err != nil {
		c.Logger.Errorf("failed to create controller token: %s", err)
	}

	// we don't want to set the output if there is an error
	if err == nil && controllerToken != "" {
		err := c.Client.Set("controllertoken", "token", strings.TrimSuffix(controllerToken, "\n"))
		if err != nil {
			c.Logger.Error(err)
		}
	}

	workerToken, err := utils.SH("k0s token create --role=worker") //nolint:errcheck
	if err != nil {
		c.Logger.Errorf("failed to create worker token: %s", err)
	}
	// we don't want to set the output if there is an error
	if err == nil && workerToken != "" {
		err := c.Client.Set("workertoken", "token", strings.TrimSuffix(workerToken, "\n"))
		if err != nil {
			c.Logger.Error(err)
		}
	}

	kubeconfig, err := utils.SH("k0s config create") //nolint:errcheck
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	if kubeconfig != "" {
		err := c.Client.Set("kubeconfig", "control-plane", base64.RawURLEncoding.EncodeToString([]byte(kubeconfig)))
		if err != nil {
			c.Logger.Error(err)
		}
	}

	return nil
}
