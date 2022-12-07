package role

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/kairos-io/kairos/pkg/config"

	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

// scheduleRoles assigns roles to nodes. Meant to be called only by leaders
func scheduleRoles(nodes []string, c *service.RoleConfig, cc *config.Config, pconfig *providerConfig.Config) error {
	rand.Seed(time.Now().Unix())

	if pconfig.Kairos.Hybrid {
		c.Logger.Info("hybrid p2p with KubeVIP enabled")
	}

	// Assign roles to nodes
	unassignedNodes, currentRoles := getRoles(c.Client, nodes)
	c.Logger.Infof("I'm the leader. My UUID is: %s.\n Current assigned roles: %+v", c.UUID, currentRoles)

	existsMaster := false

	masterRole := "master"
	workerRole := "worker"
	masterHA := "master/ha"

	if pconfig.K3s.HA.Enable {
		masterRole = "master/clusterinit"
	}
	mastersHA := 0

	for _, r := range currentRoles {
		if r == masterRole {
			existsMaster = true
		}
		if r == masterHA {
			mastersHA++
		}
	}

	c.Logger.Infof("Master already present: %t", existsMaster)
	c.Logger.Infof("Unassigned nodes: %+v", unassignedNodes)

	if !existsMaster && len(unassignedNodes) > 0 {
		var selected string
		toSelect := unassignedNodes

		// Avoid to schedule to ourselves if we have a static role
		if pconfig.Kairos.Role != "" {
			toSelect = []string{}
			for _, u := range unassignedNodes {
				if u != c.UUID {
					toSelect = append(toSelect, u)
				}
			}
		}

		// select one node without roles to become master
		if len(toSelect) == 1 {
			selected = toSelect[0]
		} else {
			selected = toSelect[rand.Intn(len(toSelect)-1)]
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

	if pconfig.K3s.HA.Enable && pconfig.K3s.HA.MasterNodes != mastersHA {
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
