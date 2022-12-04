// nolint
package mos_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
)

func sucYAML(image, version string) string {
	return `
---
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: os-upgrade
  namespace: system-upgrade
  labels:
    k3s-upgrade: server
spec:
  concurrency: 1
  version: "` + version + `"
  nodeSelector:
    matchExpressions:
      - {key: kubernetes.io/hostname, operator: Exists}
  serviceAccountName: system-upgrade
  cordon: false
  upgrade:
    image: "` + image + `"
    command:
    - "/usr/sbin/suc-upgrade"
`

}

var _ = Describe("k3s upgrade test from k8s", Label("upgrade-latest-with-kubernetes"), func() {
	containerImage := os.Getenv("CONTAINER_IMAGE")

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
			if containerImage == "" {
				Fail("CONTAINER_IMAGE needs to be set")
			}
			if isFlavor("alpine") {
				out, _ := Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
			} else {
				// Eventually(func() string {
				// 	out, _ := Machine.Command("sudo systemctl status kairos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled; vendor preset: disabled)"))
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
				// Eventually(func() string {
				// 	out, _ := Machine.Command("sudo systemctl status kairos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := Sudo("systemctl status kairos-agent")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos-agent.service; enabled; vendor preset: disabled)"))

				out, _ = Sudo("systemctl status systemd-timesyncd")
				Expect(out).Should(ContainSubstring("loaded (/usr/lib/systemd/system/systemd-timesyncd.service; enabled; vendor preset: disabled)"))
			}
		})

		It("upgrades from kubernetes", func() {
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
				out, _ := Sudo("cat /etc/rancher/k3s/k3s.yaml")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("https:"))

		

			currentVersion, err := Machine.Command("source /etc/os-release; echo $VERSION")
			Expect(err).ToNot(HaveOccurred())
			Expect(currentVersion).To(ContainSubstring("v"))

			By("wait system-upgrade-controller", func() {
				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("system-upgrade-controller"))
			})

			By("triggering an upgrade", func() {
				suc := sucYAML(strings.ReplaceAll(containerImage, ":8h", ""), "8h")

				err := ioutil.WriteFile("assets/generated.yaml", []byte(suc), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				err = Machine.SendFile("assets/generated.yaml", "./suc.yaml", "0770")
				Expect(err).ToNot(HaveOccurred())
				fmt.Println(suc)

				Eventually(func() string {
					out, _ := kubectl("apply -f suc.yaml")
					fmt.Println(out)
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("created"))

				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					fmt.Println(out)
					return out
				}, 900*time.Second, 10*time.Second).Should(ContainSubstring("apply-os-upgrade-on-"))

				Eventually(func() string {
					out, _ := kubectl("get pods -A")
					fmt.Println(out)
					version, err := Machine.Command("source /etc/os-release; echo $VERSION")
					if err != nil || !strings.Contains(version, "v") {
						// If we met error, keep going with the Eventually
						return currentVersion
					}
					return version
				}, 50*time.Minute, 10*time.Second).ShouldNot(Equal(currentVersion))
			})
		})
	})
})
