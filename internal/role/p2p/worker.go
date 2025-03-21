package role

import (
	"fmt"
	"strings"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	"github.com/kairos-io/kairos-sdk/utils"

	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/kairos-io/provider-kairos/v2/internal/role"
	common "github.com/kairos-io/provider-kairos/v2/internal/role"
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

		cpIP, _ := c.Client.Get("control-plane", "ip")
		if cpIP == "" {
			c.Logger.Info("Control plane IP not there still..")
			return nil
		}

		worker, err := NewK8sWorker(pconfig)
		if err != nil {
			return fmt.Errorf("failed to determine k8s distro: %w", err)
		}

		ip := guessIP(pconfig)
		if ip != "" {
			if err := c.Client.Set("ip", c.UUID, ip); err != nil {
				c.Logger.Error(err)
			}
		}

		worker.SetRole(common.RoleWorker)
		worker.SetRoleConfig(c)
		worker.SetIP(ip)

		workerToken, _ := worker.Token()
		if workerToken == "" {
			c.Logger.Info("worker token not there still..")
			return nil
		}

		utils.SH("kairos-agent run-stage provider-kairos.bootstrap.before.worker") //nolint:errcheck

		err = worker.SetupWorker(cpIP, workerToken)
		if err != nil {
			return err
		}

		k8sBin := worker.K8sBin()
		if k8sBin == "" {
			return fmt.Errorf("no %s binary found (?)", worker.Distro())
		}

		args, err := worker.Args()
		if err != nil {
			return err
		}

		svc, err := worker.Service()
		if err != nil {
			return err
		}

		c.Logger.Info(fmt.Sprintf("Configuring %s worker", worker.Distro()))
		if err := svc.OverrideCmd(fmt.Sprintf("%s %s %s", k8sBin, worker.Role(), strings.Join(args, " "))); err != nil {
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
