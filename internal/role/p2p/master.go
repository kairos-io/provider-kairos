package role

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil" // nolint
	"strings"
	"time"

	"github.com/kairos-io/kairos-sdk/machine"
	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/kairos-io/kairos/pkg/config"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	"github.com/kairos-io/provider-kairos/internal/role"

	service "github.com/mudler/edgevpn/api/client/service"
)

func propagateMasterData(ip string, c *service.RoleConfig, clusterInit, ha bool, role string) error {
	defer func() {
		// Avoid polluting the API.
		// The ledger already retries in the background to update the blockchain, but it has
		// a default timeout where it would stop trying afterwards.
		// Each request here would have it's own background announce, so that can become expensive
		// when network is having lot of changes on its way.
		time.Sleep(30 * time.Second)
	}()

	// If we are configured as master, always signal our role
	if err := c.Client.Set("role", c.UUID, role); err != nil {
		c.Logger.Error(err)
		return err
	}

	if ha && !clusterInit {
		return nil
	}

	tokenB, err := ioutil.ReadFile("/var/lib/rancher/k3s/server/node-token")
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

	kubeB, err := ioutil.ReadFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	kubeconfig := string(kubeB)
	if kubeconfig != "" {
		err := c.Client.Set("kubeconfig", "master", base64.RawURLEncoding.EncodeToString(kubeB))
		if err != nil {
			c.Logger.Error(err)
		}
	}
	err = c.Client.Set("master", "ip", ip)
	if err != nil {
		c.Logger.Error(err)
	}
	return nil
}

func genArgs(pconfig *providerConfig.Config, ip, ifaceIP string) (args []string) {

	if pconfig.P2P.UseVPNWithKubernetes() {
		args = append(args, "--flannel-iface=edgevpn0")
	}

	if pconfig.KubeVIP.IsEnabled() {
		args = append(args, fmt.Sprintf("--tls-san=%s", ip), fmt.Sprintf("--node-ip=%s", ifaceIP))
	}

	return
}

func genEnv(ha, clusterInit bool, c *service.Client, k3sConfig providerConfig.K3s) (env map[string]string) {
	env = make(map[string]string)

	if ha && !clusterInit {
		nodeToken, _ := c.Get("nodetoken", "token")
		env["K3S_TOKEN"] = nodeToken
	}

	if !k3sConfig.ReplaceEnv {
		// Override opts with user-supplied
		for k, v := range k3sConfig.Env {
			env[k] = v
		}
	} else {
		env = k3sConfig.Env
	}

	return env
}

// we either return the ElasticIP or the IP from the edgevpn interface.
func guessIP(pconfig *providerConfig.Config) string {
	if pconfig.KubeVIP.EIP != "" {
		return pconfig.KubeVIP.EIP
	}
	return utils.GetInterfaceIP("edgevpn0")
}

func waitForMasterHAInfo(c *service.RoleConfig) bool {
	nodeToken, _ := c.Client.Get("nodetoken", "token")
	if nodeToken == "" {
		c.Logger.Info("nodetoken not there still..")
		return true
	}
	clusterInitIP, _ := c.Client.Get("master", "ip")
	if clusterInitIP == "" {
		c.Logger.Info("clusterInitIP not there still..")
		return true
	}

	return false
}

func Master(cc *config.Config, pconfig *providerConfig.Config, clusterInit, ha bool, roleName string) role.Role {
	return func(c *service.RoleConfig) error {

		iface := guessInterface(pconfig)
		ifaceIP := utils.GetInterfaceIP(iface)
		ip := guessIP(pconfig)
		// If we don't have an IP, we sit and wait
		if ip == "" {
			return errors.New("node doesn't have an ip yet")
		}

		if pconfig.P2P.Role != "" {
			// propagate role if we were forced by configuration
			// This unblocks eventual auto instances to try to assign roles
			if err := c.Client.Set("role", c.UUID, pconfig.P2P.Role); err != nil {
				c.Logger.Error(err)
			}
		}

		if role.SentinelExist() {
			c.Logger.Info("Node already configured, backing off")
			return propagateMasterData(ip, c, clusterInit, ha, roleName)
		}

		if ha && !clusterInit && waitForMasterHAInfo(c) {
			return nil
		}

		k3sConfig := pconfig.K3s

		env := genEnv(ha, clusterInit, c.Client, k3sConfig)

		// Configure k3s service to start on edgevpn0
		c.Logger.Info("Configuring k3s")

		utils.SH(fmt.Sprintf("elemental run-stage provider-kairos.bootstrap.before.%s", roleName)) //nolint:errcheck

		svc, err := machine.K3s()
		if err != nil {
			return err
		}

		if err := utils.WriteEnv(machine.K3sEnvUnit("k3s"),
			env,
		); err != nil {
			return err
		}

		args := genArgs(pconfig, ip, ifaceIP)
		if pconfig.KubeVIP.IsEnabled() {
			if err := deployKubeVIP(iface, ip, pconfig); err != nil {
				return fmt.Errorf("failed KubeVIP setup: %w", err)
			}
		}

		if pconfig.P2P.Auto.HA.ExternalDB != "" {
			args = []string{fmt.Sprintf("--datastore-endpoint=%s", pconfig.P2P.Auto.HA.ExternalDB)}
		}

		if ha && !clusterInit {
			clusterInitIP, _ := c.Client.Get("master", "ip")
			args = append(args, fmt.Sprintf("--server=https://%s:6443", clusterInitIP))
		}

		if k3sConfig.ReplaceArgs {
			args = k3sConfig.Args
		} else {
			args = append(args, k3sConfig.Args...)
		}

		if clusterInit && ha && pconfig.P2P.Auto.HA.ExternalDB == "" {
			args = append(args, "--cluster-init")
		}

		k3sbin := utils.K3sBin()
		if k3sbin == "" {
			return fmt.Errorf("no k3s binary found (?)")
		}

		if err := svc.OverrideCmd(fmt.Sprintf("%s server %s", k3sbin, strings.Join(args, " "))); err != nil {
			return err
		}

		if err := svc.Start(); err != nil {
			return err
		}

		if err := svc.Enable(); err != nil {
			return err
		}

		if err := propagateMasterData(ip, c, clusterInit, ha, roleName); err != nil {
			return err
		}

		utils.SH(fmt.Sprintf("elemental run-stage provider-kairos.bootstrap.after.%s", roleName)) //nolint:errcheck

		return role.CreateSentinel()
	}
}
