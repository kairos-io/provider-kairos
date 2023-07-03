package mos

import (
	"fmt"
	"strings"
	"time"

	"github.com/luthermonson/go-proxmox"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func genConfig(kubeVIP string, rsa string, token string, mdns, vpnCreate, vpnUSE, kubevip bool) string {
	return fmt.Sprintf(`#cloud-config
install:
  auto: true
  device: "auto"
  reboot: true
hostname: "test-{{ trunc 4 .MachineID }}"
users:
- name: "kairos"
  passwd: "kairos"
  ssh_authorized_keys:
  - "%s"
## Sets the Elastic IP for KubeVIP
kubevip:
  eip: "%s"
  enable: %t

p2p:
  disable_dht: %t # Enabled by default
  vpn:
    create: %t # defaults to true
    use: %t # defaults to true
  auto: 
   # Auto enables automatic master/worker role assignment
   enable: true
   # HA enables automatic HA roles assignment.
   # A master cluster init is always required,
   # Any additional master_node is configured as part of the 
   # HA control plane.
   # If auto is disabled, HA has no effect.
   ha:
      # Enables HA
      enable: true
      # number of HA/master node (beside the one used for init)
      master_nodes: 2
  network_token: "%s"`, rsa, kubeVIP, kubevip, mdns, vpnCreate, vpnUSE, token)
}

func startVMS(cloudConfig []byte, number int) (VMIDS []int) {
	r := RandStringRunes(5)
	isoTestName := fmt.Sprintf("%s.iso", r)
	node, client, err := getNode()
	Expect(err).ToNot(HaveOccurred())

	storage, err := node.Storage("local")
	Expect(err).ToNot(HaveOccurred())

	err = uploadCloudInitISO(
		isoTestName,
		cloudConfig, storage,
	)
	Expect(err).ToNot(HaveOccurred())

	cluster, err := client.Cluster()
	Expect(err).ToNot(HaveOccurred())

	for i := 0; i < number; i++ {
		nextid, err := cluster.NextID()
		Expect(err).ToNot(HaveOccurred())

		// See: https://pve.proxmox.com/pve-docs/api-viewer/index.html#/nodes/{node}/qemu
		t, err := node.NewVirtualMachine(nextid,
			proxmox.VirtualMachineOption{Name: "serial0", Value: "socket"},
			proxmox.VirtualMachineOption{Name: "memory", Value: "2048"},
			proxmox.VirtualMachineOption{Name: "cores", Value: "1"},
			proxmox.VirtualMachineOption{Name: "boot", Value: "order=scsi0;ide0;net0"},
			proxmox.VirtualMachineOption{Name: "net0", Value: "virtio,bridge=vmbr0,firewall=1"},
			proxmox.VirtualMachineOption{Name: "scsi0", Value: "local-lvm:60,size=60G"},
			proxmox.VirtualMachineOption{Name: "ide0", Value: locateISOFmt(isoName())},
			proxmox.VirtualMachineOption{Name: "ide1", Value: locateISOFmt(isoTestName)},
		)
		waitTask(t, err)

		v, _ := node.VirtualMachine(nextid)
		t, err = v.Start()
		waitTask(t, err)
		VMIDS = append(VMIDS, nextid)
	}

	return
}

var _ = Describe("kairos decentralized k8s test", Label("proxmox-ha-test"), func() {

	// Used VMs ID to prune
	VMIDS := []int{}

	// ControlVM info that we need during the test run
	var dev, ip, pubkey string

	BeforeEach(func() {
		var err error
		dev, err = ControlVM.Command(`ls -l /sys/class/net/ | grep -v virtual | cut -d " " -f9`)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println("Found ControlVM dev at", dev)

		ip, err = ControlVM.Command(fmt.Sprintf(`ip a s %s | awk -F'[/ ]+' '/inet[^6]/{print $3}'`, strings.TrimSpace(dev)))
		Expect(err).ToNot(HaveOccurred(), ip)
		fmt.Println("Found ControlVM IP at", ip)

		pubkey, err = ControlVM.Command(`cat .ssh/id_rsa.pub`)
		Expect(err).ToNot(HaveOccurred())
		pubkey = strings.TrimSpace(pubkey)

		fmt.Println("Found ControlVM pubkey ", pubkey)
	})

	AfterEach(func() {
		node, _, err := getNode()
		Expect(err).ToNot(HaveOccurred())
		for _, vm := range VMIDS {
			v, err := node.VirtualMachine(vm)

			Expect(err).ToNot(HaveOccurred())
			t, err := v.Stop()
			waitTask(t, err)

			t, err = v.Delete()
			waitTask(t, err)
		}
		VMIDS = []int{}
		if CurrentGinkgoTestDescription().Failed {
			//		gatherLogs()
		}
	})

	Context("HA", func() {
		It("setups automatically an EmbeddedDB cluster with mdns", func() {
			// Get Possible IP to use for KubeVIP
			freeIP, err := ControlVM.Command(fmt.Sprintf(`my_net=%s
			for i in $(seq 1 254);
			do
			 ip="$my_net.$i"
			 ping -c2 $ip | grep -q "Unreachable"; [[ "$?" == "0" ]] && echo $ip && break
			done`, strings.Join(strings.Split(ip, ".")[0:3], ".")))
			Expect(err).ToNot(HaveOccurred(), freeIP)

			freeIP = strings.TrimSpace(freeIP)
			fmt.Println("Found Free IP at", freeIP)

			networkToken, err := genToken()
			Expect(err).ToNot(HaveOccurred())

			VMIDS = append(VMIDS, startVMS([]byte(genConfig(freeIP, pubkey, networkToken, true, false, false, true)), 4)...)

			By("Waiting for HA control-plane to be available", func() {
				ping(freeIP, ControlVM)
			})

			Eventually(func() string {
				out, err := ControlVM.Command(fmt.Sprintf("ssh -oStrictHostKeyChecking=no kairos@%s kairos role list", freeIP))
				if err != nil {
					fmt.Println(err, out)
				}
				return out
			}, time.Duration(time.Duration(650)*time.Second), time.Duration(30*time.Second)).Should(And(
				ContainSubstring("worker"),
				ContainSubstring("master/ha"),
				ContainSubstring("master/clusterinit"),
				HaveMinMaxRole("master/clusterinit", 1, 1),
				HaveMinMaxRole("master/ha", 2, 2),
			))

			out, err := ControlVM.Command(fmt.Sprintf("ssh -oStrictHostKeyChecking=no kairos@%s sudo cat /etc/systemd/system.conf.d/edgevpn-kairos.env", freeIP))
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(`EDGEVPNDHT="false"`))
		})

		It("setups automatically an EmbeddedDB cluster with dht", func() {
			out, err := ControlVM.Command("sudo cat /etc/os-release")
			Expect(err).ToNot(HaveOccurred(), out)
			if strings.Contains(out, "alpine") {
				Skip("test assumes systemd on the nodes")
			}
			networkToken, err := genToken()
			Expect(err).ToNot(HaveOccurred())

			VMIDS = append(VMIDS, startVMS([]byte(genConfig("", pubkey, networkToken, false, true, true, false)), 4)...)

			startVPN(networkToken, ControlVM)
			defer stopVPN(ControlVM)
			//

			// 10.1.0.1 will be our IP, and DHCP will assign then 10.1.0.2 to one of the nodes of the cluster.
			By("Waiting for HA control-plane to be available", func() {
				ping("10.1.0.2", ControlVM)
			})

			Eventually(func() string {
				out, err := ControlVM.Command(fmt.Sprintf("ssh -oStrictHostKeyChecking=no kairos@%s kairos role list", "10.1.0.2"))
				if err != nil {
					fmt.Println(err, out)
				}
				return out
			}, time.Duration(time.Duration(650)*time.Second), time.Duration(30*time.Second)).Should(And(
				ContainSubstring("worker"),
				ContainSubstring("master/ha"),
				ContainSubstring("master/clusterinit"),
				HaveMinMaxRole("master/clusterinit", 1, 1),
				HaveMinMaxRole("master/ha", 2, 2),
			))

			out, err = ControlVM.Command(fmt.Sprintf("ssh -oStrictHostKeyChecking=no kairos@%s sudo cat /etc/systemd/system.conf.d/edgevpn-kairos.env", "10.1.0.2"))
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(ContainSubstring(`EDGEVPNDHT="false"`))
		})
	})
})

func HaveMinMaxRole(name string, min, max int) types.GomegaMatcher {
	return WithTransform(
		func(actual interface{}) (int, error) {
			switch s := actual.(type) {
			case string:
				return strings.Count(s, name), nil
			default:
				return 0, fmt.Errorf("HaveRoles expects a string, but got %T", actual)
			}
		}, SatisfyAll(
			BeNumerically(">=", min),
			BeNumerically("<=", max)))
}
