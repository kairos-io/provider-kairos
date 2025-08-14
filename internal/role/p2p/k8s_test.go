package role

import (
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewK8sNode", func() {
	Context("explicit k8s configuration", func() {
		It("should return error when k3s is explicitly disabled", func() {
			enabled := false
			config := &providerConfig.Config{
				K3s: providerConfig.K3s{
					Enabled: &enabled,
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("k3s is explicitly disabled"))
		})

		It("should return error when k3s-agent is explicitly disabled", func() {
			enabled := false
			config := &providerConfig.Config{
				K3sAgent: providerConfig.K3s{
					Enabled: &enabled,
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("k3s-agent is explicitly disabled"))
		})

		It("should return error when k0s is explicitly disabled", func() {
			enabled := false
			config := &providerConfig.Config{
				K0s: providerConfig.K0s{
					Enabled: &enabled,
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("k0s is explicitly disabled"))
		})

		It("should return error when k0s-worker is explicitly disabled", func() {
			enabled := false
			config := &providerConfig.Config{
				K0sWorker: providerConfig.K0s{
					Enabled: &enabled,
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("k0s-worker is explicitly disabled"))
		})
	})

	Context("p2p configuration with specific role", func() {
		It("should return error for invalid p2p.role", func() {
			config := &providerConfig.Config{
				P2P: &providerConfig.P2P{
					Role: "invalid",
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid p2p.role specified"))
		})
	})

	Context("conflicting configurations", func() {
		It("should return error when both p2p.auto.enabled=true and p2p.role are specified", func() {
			enabled := true
			config := &providerConfig.Config{
				P2P: &providerConfig.P2P{
					Role: RoleMaster,
					Auto: providerConfig.Auto{
						Enable: &enabled,
					},
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot specify both p2p.auto.enabled=true and p2p.role"))
		})
	})

	Context("no k8s configuration", func() {
		It("should return error when no k8s configuration is provided", func() {
			config := &providerConfig.Config{}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no k8s configuration found"))
		})

		It("should return error when p2p is configured but no k8s component is enabled", func() {
			config := &providerConfig.Config{
				P2P: &providerConfig.P2P{
					NetworkToken: "fooblar",
				},
			}

			_, err := NewK8sNode(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("p2p is configured but no k8s component is explicitly enabled"))
		})
	})
})
