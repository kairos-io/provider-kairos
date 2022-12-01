package role

import (
	"math/rand"
	"time"

	"github.com/kairos-io/kairos/pkg/config"

	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	service "github.com/mudler/edgevpn/api/client/service"
)

// func assignIPs(cidr string, client *service.Client, nodes []string, pconfig *providerConfig.Config) (error, bool) {
// 	address, _, err := net.ParseCIDR(cidr)
// 	if err != nil {
// 		return err, false
// 	}

// 	currentIPS := []string{}
// 	toAssign := []string{}
// 	for _, a := range nodes {
// 		ip, _ := client.Get("ip", a)
// 		if ip != "" {
// 			currentIPS = append(currentIPS, ip)
// 		} else {
// 			toAssign = append(toAssign, a)
// 		}
// 	}

// 	if len(toAssign) == 0 {
// 		return nil, false
// 	}

// 	ip := utils.NextIP(address.String(), currentIPS)
// 	if err := client.Set("ip", toAssign[0], ip); err != nil {
// 		return err, false
// 	}

// 	return nil, len(toAssign) != 0
// }

// scheduleRoles assigns roles to nodes. Meant to be called only by leaders
// TODO: HA-Auto.
func scheduleRoles(nodes []string, c *service.RoleConfig, cc *config.Config, pconfig *providerConfig.Config) error {
	rand.Seed(time.Now().Unix())

	// Assign roles to nodes
	unassignedNodes, currentRoles := getRoles(c.Client, nodes)
	c.Logger.Infof("I'm the leader. My UUID is: %s.\n Current assigned roles: %+v", c.UUID, currentRoles)

	existsMaster := false

	masterRole := "master"
	workerRole := "worker"

	if pconfig.Kairos.Hybrid {
		// 	err, reschedule := assignIPs(pconfig.KubeVIP.CIDR, c.Client, nodes, pconfig)
		// 	if reschedule {
		// 		return fmt.Errorf("asked to reschedule")
		// 	}
		// 	if err != nil {
		// 		return err
		// 	}
		// 	ip, _ := c.Client.Get("ip", c.UUID)
		// 	c.Logger.Infof("KubeVIP IP: %+v", ip)
		c.Logger.Info("hybrid p2p enabled")
		//	masterRole = "kubevip/master"
		//	workerRole = "kubevip/worker"
	}

	for _, r := range currentRoles {
		if r == masterRole {
			existsMaster = true
		}
	}
	c.Logger.Infof("Master already present: %t", existsMaster)
	c.Logger.Infof("Unassigned nodes: %+v", unassignedNodes)
	selectedMaster := ""
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
		c.Logger.Info("-> Set master to", selected)
		currentRoles[selected] = masterRole
		selectedMaster = selected
		// Return here, so next time we get called
		// makes sure master is set.
		return nil
	}

	// cycle all empty roles and assign worker roles
	for _, uuid := range unassignedNodes {
		if selectedMaster == uuid {
			continue
		}
		if err := c.Client.Set("role", uuid, workerRole); err != nil {
			c.Logger.Error(err)
			return err
		}
		c.Logger.Infof("-> Set %s to %s", workerRole, uuid)
	}

	c.Logger.Info("Done scheduling")

	return nil
}
