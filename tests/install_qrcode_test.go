// nolint
package mos_test

import (
	"fmt"
	"os"
	"time"

	"github.com/kairos-io/kairos/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("kairos qr code install", Label("qrcode-install"), func() {
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
			v, _ := Machine.Command("cat /proc/cmdline")
			Expect(v).To(ContainSubstring("rd.cos.disable"))

			// sleep enough to give time to qr code to display.
			// TODO: This can be enhanced
			time.Sleep(5 * time.Minute)

			download("https://github.com/schollz/croc/releases/download/v9.6.0/croc_9.6.0_macOS-64bit.tar.gz")

			// Wait until we reboot into active, after the system is installed
			By("sharing a screenshot", func() {
				Eventually(func() error {
					file, err := screenshot()
					Expect(err).ToNot(HaveOccurred())

					defer os.RemoveAll(file)
					out, err := utils.SH(fmt.Sprintf("mv %s screenshot.png && ./croc send --code %s %s", file, os.Getenv("SENDKEY"), "screenshot.png"))
					fmt.Println(out)
					return err
				}, 10*time.Minute, 10*time.Second).ShouldNot(HaveOccurred())
			})
			By("checking that the installer is running", func() {
				Eventually(func() string {
					v, _ = Machine.Command("ps aux")
					return v
				}, 20*time.Minute, 10*time.Second).Should(ContainSubstring("elemental install"))
			})

			By("checking that the installer has terminated", func() {
				Eventually(func() string {
					v, _ = Machine.Command("ps aux")
					return v
				}, 10*time.Minute, 10*time.Second).ShouldNot(ContainSubstring("elemental install"))
			})

			By("restarting on the installed system", func() {
				detachAndReboot()

				Eventually(func() string {
					v, _ = Machine.Command("cat /proc/cmdline")
					return v
				}, 10*time.Minute, 10*time.Second).ShouldNot(ContainSubstring("rd.cos.disable"))
			})
		})
	})
})
