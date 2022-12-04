// nolint
package mos_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("k3s upgrade test", Label("upgrade-k8s"), func() {
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
				out, _ = Sudo("ps aux")
				Expect(out).Should(ContainSubstring("/usr/sbin/crond"))
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.SSHCommand("sudo systemctl status kairos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled; vendor preset: disabled)"))

				out, _ = Sudo("systemctl status logrotate.timer")
				Expect(out).Should(ContainSubstring("active (waiting)"))
			}
		})
	})

	Context("install", func() {
		It("to disk with custom config", func() {
			err := Machine.SendFile("assets/single.yaml", "/tmp/config.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, _ := Sudo("elemental install --cloud-init /tmp/config.yaml /dev/sda")
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
				out, _ := Sudo("systemctl status kairos-agent")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos-agent.service; enabled; vendor preset: disabled)"))

				out, _ = Sudo("systemctl status systemd-timesyncd")
				Expect(out).Should(ContainSubstring("loaded (/usr/lib/systemd/system/systemd-timesyncd.service; enabled; vendor preset: disabled)"))
			}
		})

		It("has kubeconfig", func() {
			Eventually(func() string {
				var out string
				if isFlavor("alpine") {
					out, _ = Sudo("cat /var/log/kairos/agent.log;cat /var/log/kairos-agent.log")
				} else {
					out, _ = Sudo("systemctl status kairos-agent")
				}
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("One time bootstrap starting"))

			Eventually(func() string {
				out, _ := Sudo("cat /var/log/kairos/agent-provider.log")
				return out
			}, 900*time.Second, 10*time.Second).Should(Or(ContainSubstring("One time bootstrap starting"), ContainSubstring("Sentinel exists")))

			Eventually(func() string {
				out, _ := Sudo("cat /etc/rancher/k3s/k3s.yaml")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("https:"))
		})

		It("rotates logs", func() {
			out, err := Sudo("logrotate -vf /etc/logrotate.d/kairos")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("log needs rotating"))
			_, err = Sudo("ls /var/log/kairos/agent-provider.log.1.gz")
			Expect(err).ToNot(HaveOccurred())
		})

		It("upgrades", func() {
			By("wait system-upgrade-controller", func() {
				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("system-upgrade-controller"))
			})
			By("applying plan", func() {

				err := Machine.SendFile("assets/suc.yaml", "./suc.yaml", "0770")
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() string {
					out, _ := kubectl("apply -f suc.yaml")
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("unchanged"))

				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					fmt.Println(out)
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("apply-os-upgrade-on-"))

				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					fmt.Println(out)
					version, _ := Machine.Command("source /etc/os-release; echo $VERSION")
					return version
				}, 30*time.Minute, 10*time.Second).Should(ContainSubstring("v"))
			})
		})
	})
})
