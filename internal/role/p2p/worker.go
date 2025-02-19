package role

import (
	"fmt"
	"strings"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	"github.com/kairos-io/kairos-sdk/utils"

	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/kairos-io/provider-kairos/v2/internal/role"
	service "github.com/mudler/edgevpn/api/client/service"
)

func Worker(cc *config.Config, pconfig *providerConfig.Config) role.Role { //nolint:revive
	return func(c *service.RoleConfig) error {
		c.Logger.Info("Starting Worker")

		if pconfig.P2P.Role != "" {
			// propagate role if we were forced by configuration
			// This unblocks eventual auto instances to try to assign roles
			if err := c.Client.Set("role", c.UUID, pconfig.P2P.Role); err != nil {
				return err
			}
		}

		if role.SentinelExist() {
			c.Logger.Info("Node already configured, backing off")
			return nil
		}

		masterIP, _ := c.Client.Get("master", "ip")
		if masterIP == "" {
			c.Logger.Info("MasterIP not there still..")
			return nil
		}

		distro := pconfig.K8sDistro()

		var node K8sNode

		if distro == providerConfig.K3sDistro {
			node = &K3sNode{roleConfig: c, providerConfig: pconfig, role: RoleWorker}
		}

		if distro == providerConfig.K0sDistro {
			node = &K0sNode{roleConfig: c, providerConfig: pconfig, role: RoleWorker}
		}

		nodeToken, _ := node.Token()
		if nodeToken == "" {
			c.Logger.Info("node token not there still..")
			return nil
		}

		utils.SH("kairos-agent run-stage provider-kairos.bootstrap.before.worker") //nolint:errcheck

		err := node.SetupWorker(masterIP, nodeToken)
		if err != nil {
			return err
		}

		k8sBin := node.K8sBin()
		if k8sBin == "" {
			return fmt.Errorf("no %s binary found (?)", distro)
		}

		args, err := node.WorkerArgs()
		if err != nil {
			return err
		}

		svc, err := node.Service()
		if err != nil {
			return err
		}

		c.Logger.Info(fmt.Sprintf("Configuring %s worker", distro))
		if err := svc.OverrideCmd(fmt.Sprintf("%s %s %s", k8sBin, node.CmdFirstArg(), strings.Join(args, " "))); err != nil {
			return err
		}

		if err := svc.Start(); err != nil {
			return err
		}

		if err := svc.Enable(); err != nil {
			return err
		}

		utils.SH("kairos-agent run-stage provider-kairos.bootstrap.after.worker") //nolint:errcheck

		return role.CreateSentinel()
	}
}
