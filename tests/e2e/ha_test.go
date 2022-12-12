package mos_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/luthermonson/go-proxmox"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func genConfig(kubeVIP string, rsa string, token string) string {
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
 manifest_url: ""
 enable: true

p2p:
 disable_dht: true # Enabled by default
 vpn:
  create: false # defaults to true
  use: false # defaults to true
 auto-ha:
  # Enables HA
  enable: true
  # number of HA/master node (beside the one used for init)
  master_nodes: 2
 network_token: "%s"`, rsa, kubeVIP, token)
}

var _ = Describe("kairos decentralized k8s test", Label("proxmox-ha-test"), func() {
	BeforeEach(func() {
		//	EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			//		gatherLogs()
		}
	})

	Context("HA", func() {
		It("setups automatically an EmbeddedDB cluster", func() {

			// Get Possible IP to use for KubeVIP
			dev, err := ControlVM.Command(`ls -l /sys/class/net/ | grep -v virtual | cut -d " " -f9`)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println("Found ControlVM dev at", dev)

			ip, err := ControlVM.Command(fmt.Sprintf(`ip a s %s | awk -F'[/ ]+' '/inet[^6]/{print $3}'`, strings.TrimSpace(dev)))
			Expect(err).ToNot(HaveOccurred(), ip)
			fmt.Println("Found ControlVM IP at", ip)

			pubkey, err := ControlVM.Command(`cat .ssh/id_rsa.pub`)
			Expect(err).ToNot(HaveOccurred())
			pubkey = strings.TrimSpace(pubkey)

			fmt.Println("Found ControlVM pubkey ", pubkey)

			freeIP, err := ControlVM.Command(fmt.Sprintf(`my_net=%s
			for i in $(seq 1 254);
			do
			 ip="$my_net.$i"
			 ping -c2 $ip | grep -q "Unreachable"; [[ "$?" == "0" ]] && echo $ip && break
			done`, strings.Join(strings.Split(ip, ".")[0:3], ".")))
			Expect(err).ToNot(HaveOccurred(), freeIP)

			freeIP = strings.TrimSpace(freeIP)
			fmt.Println("Found Free IP at", freeIP)

			r := RandStringRunes(5)
			isoTestName := fmt.Sprintf("%s.iso", r)
			node, client, err := getNode()
			Expect(err).ToNot(HaveOccurred())

			storage, err := node.Storage("local")
			Expect(err).ToNot(HaveOccurred())

			networkToken, err := genToken()
			Expect(err).ToNot(HaveOccurred())

			err = uploadCloudInitISO(
				isoTestName,
				[]byte(genConfig(freeIP, pubkey, networkToken)), storage,
			)
			Expect(err).ToNot(HaveOccurred())
			// ip a s eth0 | awk -F'[/ ]+' '/inet[^6]/{print $3}'
			cluster, err := client.Cluster()
			Expect(err).ToNot(HaveOccurred())

			for i := 0; i < 4; i++ {
				nextid, err := cluster.NextID()
				Expect(err).ToNot(HaveOccurred())

				fmt.Println("Next ID", nextid)

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
				Expect(err).ToNot(HaveOccurred())
				Expect(t).ToNot(BeNil())

				err = t.Wait(1*time.Second, 10*time.Second)
				Expect(err).ToNot(HaveOccurred())

				v, _ := node.VirtualMachine(nextid)
				v.Start()
			}

			By("Waiting for HA control-plane to be available", func() {
				Eventually(func() string {
					out, err := ControlVM.Command(fmt.Sprintf("ping %s -c 3", freeIP))
					if err != nil {
						fmt.Println(err)
					}
					return out
				}, time.Duration(time.Duration(650)*time.Second), time.Duration(30*time.Second)).Should(ContainSubstring("3 received"))
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
