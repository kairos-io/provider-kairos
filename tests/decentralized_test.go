// nolint
package mos

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("kairos decentralized k8s test", Label("decentralized-k8s"), func() {
	var vms []VM
	var configPath string

	BeforeEach(func() {
		iso := os.Getenv("ISO")
		_, vm1 := startVM(iso)
		_, vm2 := startVM(iso)
		vms = append(vms, vm1, vm2)

		configPath = cloudConfig()

		vmForEach(vms, func(vm VM) {
			By("waiting until ssh is possible")
			vm.EventuallyConnects(1200)
		})
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			gatherLogs(vms[0])
		}
		vmForEach(vms, func(vm VM) {
			vm.Destroy(nil)
		})
		os.RemoveAll(configPath)
	})

	It("installs to disk with custom config", func() {
		vmForEach(vms, func(vm VM) {
			By("checking if it has default service active")
			if isFlavor(vm, "alpine") {
				out, _ := vm.Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
			} else {
				out, _ := vm.Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled; vendor preset: disabled)"))
			}

			By("installing")
			err := vm.Scp(configPath, "/tmp/config.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, err := vm.Sudo("kairos-agent manual-install --device auto /tmp/config.yaml")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).Should(ContainSubstring("Running after-install hook"), out)

			out, err = vm.Sudo("sync")
			Expect(err).ToNot(HaveOccurred(), out)

			vm.Reboot()

			By("checking default services are on after first boot")
			if isFlavor(vm, "alpine") {
				Eventually(func() string {
					out, _ := vm.Sudo("rc-status")
					return out
				}, 30*time.Second, 10*time.Second).Should(And(
					ContainSubstring("kairos"),
					ContainSubstring("kairos-agent")))
			} else {
				Eventually(func() string {
					out, _ := vm.Sudo("systemctl status kairos-agent")
					return out
				}, 30*time.Second, 10*time.Second).Should(ContainSubstring(
					"loaded (/etc/systemd/system/kairos-agent.service; enabled; vendor preset: disabled)"))

				Eventually(func() string {
					out, _ := vm.Sudo("systemctl status systemd-timesyncd")
					return out
				}, 30*time.Second, 10*time.Second).Should(ContainSubstring(
					"loaded (/usr/lib/systemd/system/systemd-timesyncd.service; enabled; vendor preset: disabled)"))
			}

			if !isFlavor(vm, "alpine") {
				By("checking if it has correct grub menu entries")
				state, _ := vm.Sudo("blkid -L COS_STATE")
				state = strings.TrimSpace(state)
				out, err := vm.Sudo("blkid")
				Expect(err).ToNot(HaveOccurred(), out)
				out, err = vm.Sudo("mkdir -p /tmp/mnt/STATE")
				Expect(err).ToNot(HaveOccurred(), out)
				out, err = vm.Sudo("mount " + state + " /tmp/mnt/STATE")
				Expect(err).ToNot(HaveOccurred(), out)
				out, err = vm.Sudo("cat /tmp/mnt/STATE/grubmenu")
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).Should(ContainSubstring("Kairos remote recovery"))

				grub, err := vm.Sudo("cat /tmp/mnt/STATE/grub_oem_env")
				Expect(err).ToNot(HaveOccurred(), grub)

				Expect(grub).Should(ContainSubstring("default_menu_entry=Kairos"))

				out, err = vm.Sudo("umount /tmp/mnt/STATE")
				Expect(err).ToNot(HaveOccurred(), out)
			}

			By("checking if k3s was configured")
			out, err = vm.Sudo("cat /run/cos/live_mode")
			Expect(err).To(HaveOccurred(), out)
			if isFlavor(vm, "alpine") {
				Eventually(func() string {
					out, _ = vm.Sudo("sudo cat /var/log/kairos/agent.log")
					return out
				}, 20*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					), out)
			} else {
				Eventually(func() string {
					out, _ = vm.Sudo("systemctl status kairos-agent")
					return out
				}, 30*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					), out)
			}

			By("checking if it has default image sizes")
			for _, p := range []string{"active.img", "passive.img"} {
				out, err := vm.Sudo(`stat -c "%s" /run/initramfs/cos-state/cOS/` + p)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).Should(ContainSubstring("3145728000"))
			}

			By("checking if it has a working kubeconfig")
			Eventually(func() string {
				out, _ = vm.Sudo("kairos get-kubeconfig")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("https:"), out)

			Eventually(func() string {
				vm.Sudo("kairos get-kubeconfig > kubeconfig")
				out, _ = vm.Sudo("KUBECONFIG=kubeconfig kubectl get nodes -o wide")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("Ready"), out)

			By("checking roles")
			uuid, err := vm.Sudo("kairos-agent uuid")
			Expect(err).ToNot(HaveOccurred(), uuid)
			Expect(uuid).ToNot(Equal(""))
			Eventually(func() string {
				out, _ = vm.Sudo("kairos role list")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring(uuid),
				ContainSubstring("worker"),
				ContainSubstring("master"),
				HaveMinMaxRole("master", 1, 1),
				HaveMinMaxRole("worker", 1, 1),
			), out)

			By("checking if it has machines with different IPs")
			Eventually(func() string {
				out, _ = vm.Sudo(`curl http://localhost:8080/api/machines`)
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("10.1.0.1"),
				ContainSubstring("10.1.0.2"),
			), out)

			By("checking if it can propagate dns and it is functional")
			if !isFlavor(vm, "alpine") {
				// FIXUP: DNS needs reboot to take effect
				vm.Reboot()
				out := ""
				Eventually(func() string {
					vm.Sudo(`curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'`)
					out, _ = vm.Sudo("ping -c 1 foo.bar")
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("2.2.2.2"), out)
				Eventually(func() string {
					out, _ = vm.Sudo("ping -c 1 google.com")
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("64 bytes from"), out)
			}
		})

		// Now that both VMs are in the same state, try the upgrade
		vmForEach(vms, func(vm VM) {
			By("checking if it upgrades to a specific version")
			version, err := vm.Sudo("source /etc/os-release; echo $VERSION")
			Expect(err).ToNot(HaveOccurred(), version)

			out, err := vm.Sudo("kairos-agent upgrade --image quay.io/kairos/kairos-opensuse:v1.0.0-rc2-k3sv1.21.14-k3s1")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Upgrade completed"))

			out, err = vm.Sudo("sync")
			Expect(err).ToNot(HaveOccurred(), out)

			vm.Reboot(1200)

			version2, err := vm.Sudo(getVersionCmd)
			Expect(err).ToNot(HaveOccurred(), version2)
			Expect(version).ToNot(Equal(version2))
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

func vmForEach(vms []VM, action func(vm VM)) {
	var wg sync.WaitGroup
	for _, vm := range vms {
		wg.Add(1)
		go func(actionVM VM) {
			defer GinkgoRecover()
			defer wg.Done()
			action(actionVM)
		}(vm)
	}

	wg.Wait()
}

func cloudConfig() string {
	token, err := kairosCli("generate-token")
	Expect(err).ToNot(HaveOccurred())

	configBytes, err := os.ReadFile("assets/config.yaml")
	Expect(err).ToNot(HaveOccurred())

	config := fmt.Sprintf(`%s

p2p:
  network_token: %s
  dns: true
`, string(configBytes), token)

	f, err := os.CreateTemp("", "kairos-config-*.yaml")
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()

	_, err = f.WriteString(config)
	Expect(err).ToNot(HaveOccurred())

	return f.Name()
}
