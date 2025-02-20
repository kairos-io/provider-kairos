package role

import (
	"encoding/base64"
	"errors"
	"os"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

const (
	K0sMasterName = "controller"
	K0sWorkerName = "worker"
)

type K0sNode struct {
	providerConfig *providerConfig.Config
	roleConfig     *service.RoleConfig
	ip             string
	role           string
}

func (k *K0sNode) IsWorker() bool {
	return k.role == RoleWorker
}

func (k *K0sNode) K8sBin() string {
	return utils.K0sBin()
}

func (k *K0sNode) DeployKubeVIP() error {
	pconfig := k.ProviderConfig()
	if pconfig.KubeVIP.IsEnabled() {
		return errors.New("KubeVIP is not yet supported with k0s")
	}

	return nil
}

func (k *K0sNode) GenArgs() ([]string, error) {
	var args []string

	pconfig := k.ProviderConfig()
	if !pconfig.P2P.UseVPNWithKubernetes() {
		return args, errors.New("Having a VPN but not using it for Kubernetes is not yet supported with k0s")
	}

	if pconfig.KubeVIP.IsEnabled() {
		return args, errors.New("KubeVIP is not yet supported with k0s")
	}

	if pconfig.P2P.Auto.HA.ExternalDB != "" {
		return args, errors.New("ExternalDB is not yet supported with k0s")
	}

	if k.HA() && !k.ClusterInit() {
		return args, errors.New("HA is not yet supported with k0s")
	}

	// when we start implementing this functionality, remember to use
	// AppendArgs, and not just return the args here, this is because the
	// function understands if it needs to append or replace the args

	return args, nil
}

func (k *K0sNode) EnvUnit() string {
	return machine.K0sEnvUnit("k0scontroller")
}

func (k *K0sNode) Service() (machine.Service, error) {
	if k.IsWorker() {
		return machine.K0sWorker()
	}

	return machine.K0s()
}

func (k *K0sNode) Token() (string, error) {
	if k.IsWorker() {
		return k.RoleConfig().Client.Get("workertoken", "token")
	}

	return k.RoleConfig().Client.Get("controllertoken", "token")
}

func (k *K0sNode) GenerateEnv() (env map[string]string) {
	env = make(map[string]string)

	if k.HA() && !k.ClusterInit() {
		nodeToken, _ := k.Token()
		env["K0S_TOKEN"] = nodeToken
	}

	pConfig := k.ProviderConfig()

	if pConfig.K0s.ReplaceEnv {
		env = pConfig.K0s.Env
	} else {
		// Override opts with user-supplied
		for k, v := range pConfig.K0s.Env {
			env[k] = v
		}
	}

	return env
}

func (k *K0sNode) SetProviderConfig(c *providerConfig.Config) {
	k.providerConfig = c
}

func (k *K0sNode) ProviderConfig() *providerConfig.Config {
	return k.providerConfig
}

func (k *K0sNode) SetRoleConfig(c *service.RoleConfig) {
	k.roleConfig = c
}

func (k *K0sNode) RoleConfig() *service.RoleConfig {
	return k.roleConfig
}

func (k *K0sNode) HA() bool {
	return k.role == RoleMasterHA
}

func (k *K0sNode) ClusterInit() bool {
	// k0s does not have a cluster init role like k3s. Instead we should have a way to set in the config
	// if the user wants a single node cluster, multi-node cluster, or HA cluster
	return false
}

func (k *K0sNode) IP() string {
	return k.ip
}

func (k *K0sNode) propagateData() error {
	c := k.RoleConfig()
	controllerToken, err := utils.SH("k0s token create --role=controller") //nolint:errcheck
	if err != nil {
		c.Logger.Errorf("failed to create controller token: %s", err)
	}

	// we don't want to set the output if there is an error
	if err == nil && controllerToken != "" {
		err := c.Client.Set("controllertoken", "token", controllerToken)
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
		err := c.Client.Set("workertoken", "token", workerToken)
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
		err := c.Client.Set("kubeconfig", "master", base64.RawURLEncoding.EncodeToString([]byte(kubeconfig)))
		if err != nil {
			c.Logger.Error(err)
		}
	}

	return nil
}

func (k *K0sNode) WorkerArgs() ([]string, error) {
	pconfig := k.ProviderConfig()
	k0sConfig := pconfig.K0sWorker
	args := []string{"--token-file /etc/k0s/token"}

	if k0sConfig.ReplaceArgs {
		args = k0sConfig.Args
	} else {
		args = append(args, k0sConfig.Args...)
	}

	return args, nil
}

func (k *K0sNode) SetupWorker(_, nodeToken string) error {
	if err := os.WriteFile("/etc/k0s/token", []byte(nodeToken), 0644); err != nil {
		return err
	}

	return nil
}

func (k *K0sNode) CmdFirstArg() string {
	if k.IsWorker() {
		return K0sWorkerName
	}

	return K0sMasterName
}
