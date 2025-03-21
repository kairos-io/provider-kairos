package role

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	"github.com/kairos-io/kairos-sdk/utils"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/kairos-io/provider-kairos/v2/internal/role"

	service "github.com/mudler/edgevpn/api/client/service"
)

func propagateControlPlaneData(role string, k K8sControlPlane) error {
	c := k.RoleConfig()
	defer func() {
		// Avoid polluting the API.
		// The ledger already retries in the background to update the blockchain, but it has
		// a default timeout where it would stop trying afterwards.
		// Each request here would have it's own background announce, so that can become expensive
		// when network is having lot of changes on its way.
		time.Sleep(30 * time.Second)
	}()

	// If we are configured as control plane, always signal our role
	if err := c.Client.Set("role", c.UUID, role); err != nil {
		c.Logger.Error(err)
		return err
	}

	if k.HA() && !k.ClusterInit() {
		return nil
	}

	err := k.PropagateData()
	if err != nil {
		c.Logger.Error(err)
	}

	err = c.Client.Set("control-plane", "ip", k.IP())
	if err != nil {
		c.Logger.Error(err)
	}
	return nil
}

// we either return the ElasticIP or the IP from the edgevpn interface.
func guessIP(pconfig *providerConfig.Config) string {
	if pconfig.KubeVIP.EIP != "" {
		return pconfig.KubeVIP.EIP
	}
	return utils.GetInterfaceIP("edgevpn0")
}

func waitForControlPlaneHAInfo(m K8sControlPlane) bool {
	var controlPlaneToken string

	controlPlaneToken, _ = m.Token()
	c := m.RoleConfig()

	if controlPlaneToken == "" {
		c.Logger.Info("Control Plane's token is not there yet..")
		return true
	}
	clusterInitIP, _ := c.Client.Get("control-plane", "ip")
	if clusterInitIP == "" {
		c.Logger.Info("the clusterInitIP is not there yet..")
		return true
	}

	return false
}

func ControlPlane(cc *config.Config, pconfig *providerConfig.Config, roleName string) role.Role { //nolint:revive
	return func(c *service.RoleConfig) error {
		c.Logger.Info(fmt.Sprintf("Starting ControlPlane(%s)", roleName))

		ip := guessIP(pconfig)
		// If we don't have an IP, we sit and wait
		if ip == "" {
			return errors.New("control plane doesn't have an ip yet")
		}

		c.Logger.Info("Checking role assignment")

		if pconfig.P2P.Role != "" {
			c.Logger.Info(fmt.Sprintf("Setting role from configuration: %s", pconfig.P2P.Role))
			// propagate role if we were forced by configuration
			// This unblocks eventual auto instances to try to assign roles
			if err := c.Client.Set("role", c.UUID, pconfig.P2P.Role); err != nil {
				c.Logger.Error(err)
			}
		}

		c.Logger.Info("Determining K8s distro")
		controlPlane, err := NewK8sControlPlane(pconfig)
		if err != nil {
			return fmt.Errorf("failed to determine k8s distro: %w", err)
		}

		controlPlane.SetRole(roleName)
		controlPlane.SetRoleConfig(c)
		controlPlane.SetIP(ip)
		controlPlane.GuessInterface()

		c.Logger.Info("Verifying sentinel file")
		if role.SentinelExist() {
			c.Logger.Info("Control Plane already configured, propagating control plane data and backing off")
			return propagateControlPlaneData(roleName, controlPlane)
		}

		c.Logger.Info("Checking HA")
		if controlPlane.HA() && !controlPlane.ClusterInit() && waitForControlPlaneHAInfo(controlPlane) {
			return nil
		}

		c.Logger.Info("Generating env")
		env := controlPlane.GenerateEnv()

		// Configure k8s service to start on edgevpn0
		c.Logger.Info(fmt.Sprintf("Configuring %s", controlPlane.Distro()))

		c.Logger.Info("Running bootstrap before stage")
		utils.SH(fmt.Sprintf("kairos-agent run-stage provider-kairos.bootstrap.before.%s", roleName)) //nolint:errcheck

		if controlPlane.HA() {
			err = controlPlane.SetupHAToken()
			if err != nil {
				return err
			}
		}

		svc, err := controlPlane.Service()
		if err != nil {
			return fmt.Errorf("failed to get %s service: %w", controlPlane.Distro(), err)
		}

		c.Logger.Info("Writing service Env %s")
		envUnit := controlPlane.EnvUnit()
		if err := utils.WriteEnv(envUnit,
			env,
		); err != nil {
			return fmt.Errorf("failed to write the %s service: %w", controlPlane.Distro(), err)
		}

		c.Logger.Info("Generating args")
		args, err := controlPlane.Args()
		if err != nil {
			return fmt.Errorf("failed to generate %s args: %w", controlPlane.Distro(), err)
		}

		if controlPlane.ProviderConfig().KubeVIP.IsEnabled() {
			c.Logger.Info("Configuring KubeVIP")
			if err := controlPlane.DeployKubeVIP(); err != nil {
				return fmt.Errorf("failed KubeVIP setup: %w", err)
			}
		}

		k8sBin := controlPlane.K8sBin()
		if k8sBin == "" {
			return fmt.Errorf("no %s binary found (?)", controlPlane.Distro())
		}

		c.Logger.Info("Writing service override")
		if err := svc.OverrideCmd(fmt.Sprintf("%s %s %s", k8sBin, controlPlane.Role(), strings.Join(args, " "))); err != nil {
			return fmt.Errorf("failed to override %s command: %w", controlPlane.Distro(), err)
		}

		c.Logger.Info("Starting service")
		if err := svc.Start(); err != nil {
			return fmt.Errorf("failed to start %s service: %w", controlPlane.Distro(), err)
		}

		c.Logger.Info("Enabling service")
		if err := svc.Enable(); err != nil {
			return fmt.Errorf("failed to enable %s service: %w", controlPlane.Distro(), err)
		}

		c.Logger.Info("Propagating control plane data")
		if err := propagateControlPlaneData(roleName, controlPlane); err != nil {
			return fmt.Errorf("failed to propagate control plane data: %w", err)
		}

		c.Logger.Info("Running after bootstrap stage")
		utils.SH(fmt.Sprintf("kairos-agent run-stage provider-kairos.bootstrap.after.%s", roleName)) //nolint:errcheck

		c.Logger.Info("Creating sentinel")
		if err := role.CreateSentinel(); err != nil {
			return fmt.Errorf("failed to create sentinel: %w", err)
		}

		return nil
	}
}
