package role

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/mudler/edgevpn/api/client/service"
	"github.com/samber/lo"
)

// scheduleRoles assigns roles to nodes. Meant to be called only by leaders.
func scheduleRoles(nodes []string, c *service.RoleConfig, cc *config.Config, pconfig *providerConfig.Config) error { //nolint:revive
	// From the golang docs: https://pkg.go.dev/math/rand#example-package-Rand
	// Create and seed the generator.
	// Typically a non-fixed seed should be used, such as time.Now().UnixNano().
	// Using a fixed seed will produce the same output on every run.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

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

	existsMaster := false

	masterRole := "master"
	workerRole := "worker"
	masterHA := "master/ha"

	if pconfig.P2P.Auto.HA.IsEnabled() {
		masterRole = "master/clusterinit"
	}
	mastersHA := 0

	for _, r := range currentRoles {
		switch r {
		case masterRole:
			existsMaster = true
		case masterHA:
			mastersHA++
		}
	}

	c.Logger.Infof("Master already present: %t", existsMaster)
	c.Logger.Infof("Unassigned nodes: %+v", unassignedNodes)

	if !existsMaster && len(unassignedNodes) > 0 {
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

		// select one node without roles to become master
		if len(toSelect) == 0 {
			// No nodes available for selection (all filtered out)
			c.Logger.Warnf("No nodes available for master selection after filtering")
			return nil
		} else if len(toSelect) == 1 {
			selected = toSelect[0]
		} else {
			selected = toSelect[r.Intn(len(toSelect))]
		}

		if err := c.Client.Set("role", selected, masterRole); err != nil {
			return err
		}
		c.Logger.Infof("-> Set %s to %s", masterRole, selected)
		currentRoles[selected] = masterRole
		// Return here, so next time we get called
		// makes sure master is set.
		return nil
	}

	if pconfig.P2P.Auto.HA.IsEnabled() && pconfig.P2P.Auto.HA.MasterNodes != nil && *pconfig.P2P.Auto.HA.MasterNodes != mastersHA {
		if len(unassignedNodes) > 0 {
			if err := c.Client.Set("role", unassignedNodes[0], masterHA); err != nil {
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
		if err := c.Client.Set("role", uuid, workerRole); err != nil {
			c.Logger.Error(err)
			return err
		}
		c.Logger.Infof("-> Set %s to %s", workerRole, uuid)
	}

	c.Logger.Info("Done scheduling")

	return nil
}
