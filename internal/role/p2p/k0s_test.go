package role

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("k0s configuration", func() {
	It("preserves user values while applying provider settings", func() {
		defaults := []byte(`
spec:
  api:
    address: 192.0.2.10
  network:
    provider: kuberouter
    kuberouter:
      metricsPort: 8080
  storage:
    type: etcd
    etcd:
      peerAddress: 192.0.2.10
`)
		user := []byte(`
spec:
  api:
    sans:
      - api.example.com
`)

		data, err := buildK0sConfig(defaults, user, "10.0.0.2")
		Expect(err).NotTo(HaveOccurred())

		var config map[string]any
		Expect(yaml.Unmarshal(data, &config)).To(Succeed())
		spec := config["spec"].(map[string]any)
		api := spec["api"].(map[string]any)
		network := spec["network"].(map[string]any)
		kubeRouter := network["kuberouter"].(map[string]any)
		storage := spec["storage"].(map[string]any)
		etcd := storage["etcd"].(map[string]any)

		Expect(api["sans"]).To(Equal([]any{"api.example.com"}))
		Expect(api["address"]).To(Equal("10.0.0.2"))
		Expect(network["provider"]).To(Equal("kuberouter"))
		Expect(kubeRouter["metricsPort"]).To(Equal(9090))
		Expect(storage["type"]).To(Equal("etcd"))
		Expect(etcd["peerAddress"]).To(Equal("10.0.0.2"))
	})
})
