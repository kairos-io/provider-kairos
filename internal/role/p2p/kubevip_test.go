package role

import (
	"fmt"
	"os"

	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/kube-vip/kube-vip/pkg/bgp"
	"github.com/kube-vip/kube-vip/pkg/kubevip"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KubeVIP Generation", func() {
	Context("generateKubeVIP with embedded config", func() {
		It("generates daemonset manifest with embedded kubevip.Config values", func() {
			enable := true
			config := &providerConfig.Config{
				KubeVIP: providerConfig.KubeVIP{
					EIP:       "192.168.1.100",
					Enable:    &enable,
					Interface: "eth0",
					Version:   "v1.0.1",
					Config: kubevip.Config{
						EnableServices:     true,
						EnableARP:          true,
						EnableBGP:          false,
						EnableControlPlane: false, // Will be overridden by generateKubeVIP
						VIP:                "192.168.1.100",
					},
				},
			}

			content, err := generateKubeVIP("daemonset", "eth0", "192.168.1.100", config)

			Expect(err).ToNot(HaveOccurred())
			Expect(content).ToNot(BeEmpty())
			Expect(content).To(ContainSubstring("kind: DaemonSet"))
			Expect(content).To(ContainSubstring("kube-vip"))

			// Print the YAML content to stdout so it's clearly visible
			fmt.Fprintf(os.Stdout, "\n=== Generated KubeVIP DaemonSet Manifest ===\n")
			fmt.Fprintf(os.Stdout, "%s\n", content)
			fmt.Fprintf(os.Stdout, "=== End of Manifest ===\n\n")
		})

		It("generates pod manifest with embedded kubevip.Config values", func() {
			enable := true
			config := &providerConfig.Config{
				KubeVIP: providerConfig.KubeVIP{
					EIP:       "10.0.0.50",
					Enable:    &enable,
					Interface: "ens3",
					StaticPod: true,
					Version:   "v1.0.1",
					Config: kubevip.Config{
						EnableServices:       true,
						EnableARP:            true,
						VIP:                  "10.0.0.50",
						PrometheusHTTPServer: ":2112",
					},
				},
			}

			content, err := generateKubeVIP("pod", "ens3", "10.0.0.50", config)

			Expect(err).ToNot(HaveOccurred())
			Expect(content).ToNot(BeEmpty())
			Expect(content).To(ContainSubstring("kind: Pod"))
			Expect(content).To(ContainSubstring("kube-vip"))

			// Print the YAML content to stdout so it's clearly visible
			fmt.Fprintf(os.Stdout, "\n=== Generated KubeVIP Pod Manifest ===\n")
			fmt.Fprintf(os.Stdout, "%s\n", content)
			fmt.Fprintf(os.Stdout, "=== End of Manifest ===\n\n")
		})

		It("applies embedded config fields correctly", func() {
			enable := true
			config := &providerConfig.Config{
				KubeVIP: providerConfig.KubeVIP{
					EIP:    "172.16.0.10",
					Enable: &enable,
					Config: kubevip.Config{
						EnableServices: true,
						EnableARP:      true,
						EnableBGP:      true,
						BGPConfig: bgp.Config{
							RouterID: "192.168.1.1",
							AS:       65000,
						},
						VIP: "172.16.0.10",
					},
				},
			}

			content, err := generateKubeVIP("daemonset", "eth1", "172.16.0.10", config)

			Expect(err).ToNot(HaveOccurred())
			Expect(content).ToNot(BeEmpty())

			// Print the YAML content to stdout so it's clearly visible
			fmt.Fprintf(os.Stdout, "\n=== Generated KubeVIP Manifest with BGP ===\n")
			fmt.Fprintf(os.Stdout, "%s\n", content)
			fmt.Fprintf(os.Stdout, "=== End of Manifest ===\n\n")
		})
	})
})
