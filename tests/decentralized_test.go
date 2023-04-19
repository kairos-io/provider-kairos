// nolint
package mos

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("kairos decentralized k8s test", Label("decentralized-k8s"), func() {
	BeforeEach(func() {
		EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			gatherLogs()
		}
	})

	Context("live cd", func() {
		It("has default service active", func() {
			if isFlavor("alpine") {
				out, _ := Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.SSHCommand("sudo systemctl status kairos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled; vendor preset: disabled)"))
			}
		})
	})

	Context("install", func() {
		It("to disk with custom config", func() {
			err := Machine.SendFile(os.Getenv("CLOUD_INIT"), "/tmp/config.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, _ := Sudo("kairos-agent manual-install --device auto /tmp/config.yaml")
			Expect(out).Should(ContainSubstring("Running after-install hook"))
			fmt.Println(out)
			Sudo("sync")
			detachAndReboot()
		})
	})

	Context("first-boot", func() {

		It("has default services on", func() {
			if isFlavor("alpine") {
				out, _ := Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.SSHCommand("sudo systemctl status kairos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := Sudo("systemctl status kairos-agent")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos-agent.service; enabled; vendor preset: disabled)"))

				out, _ = Sudo("systemctl status systemd-timesyncd")
				Expect(out).Should(ContainSubstring("loaded (/usr/lib/systemd/system/systemd-timesyncd.service; enabled; vendor preset: disabled)"))
			}
		})

		It("has correct grub menu entries", func() {
			if isFlavor("alpine") {
				Skip("not working on alpine yet")
			}

			By("checking entries", func() {
				state, _ := Sudo("blkid -L COS_STATE")
				state = strings.TrimSpace(state)
				out, _ := Sudo("blkid")
				fmt.Println(out)
				out, _ = Sudo("mkdir -p /tmp/mnt/STATE")
				fmt.Println(out)
				out, _ = Sudo("mount " + state + " /tmp/mnt/STATE")
				fmt.Println(out)
				out, _ = Sudo("cat /tmp/mnt/STATE/grubmenu")
				Expect(out).Should(ContainSubstring("Kairos remote recovery"))

				grub, _ := Sudo("cat /tmp/mnt/STATE/grub_oem_env")
				Expect(grub).Should(ContainSubstring("default_menu_entry=Kairos"))

				Sudo("umount /tmp/mnt/STATE")
			})
		})

		It("configure k3s", func() {
			_, err := Machine.Command("cat /run/cos/live_mode")
			Expect(err).To(HaveOccurred())
			if isFlavor("alpine") {
				Eventually(func() string {
					out, _ := Sudo("sudo cat /var/log/kairos/agent.log")
					fmt.Println(out)
					return out
				}, 20*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					))
			} else {
				Eventually(func() string {
					out, _ := Sudo("systemctl status kairos-agent")
					return out
				}, 30*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					))
			}
		})

		PIt("configure edgevpn", func() {
			Eventually(func() string {
				out, _ := Sudo("cat /etc/systemd/system.conf.d/edgevpn-kairos.env")
				return out
			}, 1*time.Minute, 1*time.Second).Should(
				And(
					ContainSubstring("EDGEVPNLOGLEVEL=\"debug\""),
				))
		})

		It("has default image sizes", func() {
			for _, p := range []string{"active.img", "passive.img"} {
				out, _ := Sudo(`stat -c "%s" /run/initramfs/cos-state/cOS/` + p)
				Expect(out).Should(ContainSubstring("3145728000"))
			}
		})

		It("propagate kubeconfig", func() {
			Eventually(func() string {
				out, _ := Machine.Command("kairos get-kubeconfig")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("https:"))

			Eventually(func() string {
				Machine.Command("kairos get-kubeconfig > kubeconfig")
				out, _ := Machine.Command("KUBECONFIG=kubeconfig kubectl get nodes -o wide")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("Ready"))
		})

		It("has roles", func() {
			uuid, _ := Machine.Command("kairos-agent uuid")
			Expect(uuid).ToNot(Equal(""))
			Eventually(func() string {
				out, _ := Machine.Command("kairos role list")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring(uuid),
				ContainSubstring("worker"),
				ContainSubstring("master"),
				HaveMinMaxRole("master", 1, 1),
				HaveMinMaxRole("worker", 1, 1),
			))
		})

		It("has machines with different IPs", func() {
			Eventually(func() string {
				out, _ := Machine.Command(`curl http://localhost:8080/api/machines`)
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("10.1.0.1"),
				ContainSubstring("10.1.0.2"),
			))
		})

		It("can propagate dns and it is functional", func() {
			if isFlavor("alpine") {
				Skip("DNS not working on alpine yet")
			}
			// FIXUP: DNS needs reboot to take effect
			Reboot()
			Eventually(func() string {
				Machine.Command(`curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'`)
				out, _ := Machine.Command("ping -c 1 foo.bar")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("2.2.2.2"),
			))
			Eventually(func() string {
				out, _ := Machine.Command("ping -c 1 google.com")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("64 bytes from"),
			))
		})

		It("upgrades to a specific version", func() {
			version, _ := Machine.Command(getVersionCmd)

			out, _ := Sudo("kairos-agent upgrade --image quay.io/kairos/kairos-opensuse:v1.0.0-rc2-k3sv1.21.14-k3s1")
			Expect(out).To(ContainSubstring("Upgrade completed"))

			Sudo("sync")
			Reboot()

			EventuallyConnects(700)

			version2, _ := Machine.Command(getVersionCmd)
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
