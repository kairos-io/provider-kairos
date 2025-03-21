package role

import (
	"fmt"
	"math/rand"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/mudler/edgevpn/api/client/service"
	"github.com/samber/lo"
)

// scheduleRoles assigns roles to nodes. Meant to be called only by leaders.
func scheduleRoles(nodes []string, c *service.RoleConfig, cc *config.Config, pconfig *providerConfig.Config) error { //nolint:revive
	// Assign roles to nodes
	unassignedNodes, currentRoles := getRoles(c.Client, nodes)
	c.Logger.Infof("I'm the leader. My UUID is: %s.\n Current assigned roles: %+v", c.UUID, currentRoles)

	// Scan for dead nodes
	if pconfig.P2P.DynamicRoles {
		advertizing, _ := c.Client.AdvertizingNodes()
		for u, r := range currentRoles {
			if !lo.Contains(advertizing, u) {
				c.Logger.Infof("Role '%s' assigned to unreachable node '%s'. Unassigning.", u, r)
				if err := c.Client.Delete("role", u); err != nil {
					c.Logger.Warnf("Error announcing deletion %+v", err)
				}
				// Return here to propagate announces and wait until the map is pruned
				return nil
			}
		}
	}

	hasControlPlane := false

	controlPlaneRole := "master"

	if pconfig.P2P.Auto.HA.IsEnabled() {
		controlPlaneRole = "master/clusterinit"
	}
	controlPlaneCounter := 1 // Start at 1 to account for the init node

	for _, r := range currentRoles {
		switch r {
		case controlPlaneRole:
			hasControlPlane = true
		case RoleControlPlaneHA:
			controlPlaneCounter++
		}
	}

	c.Logger.Infof("Control Plane already present: %t", hasControlPlane)
	c.Logger.Infof("Unassigned nodes: %+v", unassignedNodes)

	if !hasControlPlane && len(unassignedNodes) > 0 {
		var selected string
		toSelect := unassignedNodes

		// Avoid to schedule to ourselves if we have a static role
		if pconfig.P2P.Role != "" {
			toSelect = []string{}
			for _, u := range unassignedNodes {
				if u != c.UUID {
					toSelect = append(toSelect, u)
				}
			}
		}

		// select one node without roles to become control-plane
		if len(toSelect) == 1 {
			selected = toSelect[0]
		} else {
			selected = toSelect[rand.Intn(len(toSelect)-1)]
		}

		if err := c.Client.Set("role", selected, controlPlaneRole); err != nil {
			return err
		}
		c.Logger.Infof("-> Set %s to %s", controlPlaneRole, selected)
		currentRoles[selected] = controlPlaneRole
		// Return here, so next time we get called
		// makes sure master is set.
		return nil
	}

	// Check if we need more control planes for HA
	if pconfig.P2P.Auto.HA.IsEnabled() && pconfig.P2P.Auto.HA.HasControlPlanes() && pconfig.P2P.Auto.HA.GetControlPlanes() != controlPlaneCounter {
		if len(unassignedNodes) > 0 {
			if err := c.Client.Set("role", unassignedNodes[0], RoleControlPlaneHA); err != nil {
				c.Logger.Error(err)
				return err
			}
			// We want to keep scheduling in a second batch
			return nil
		}
		return fmt.Errorf("not enough nodes to create HA control plane")
	}

	// cycle all empty roles and assign worker roles
	for _, uuid := range unassignedNodes {
		if err := c.Client.Set("role", uuid, RoleWorker); err != nil {
			c.Logger.Error(err)
			return err
		}
		c.Logger.Infof("-> Set %s to %s", RoleWorker, uuid)
	}

	c.Logger.Info("Done scheduling")

	return nil
}
