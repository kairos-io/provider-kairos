package cli_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kairos-io/kairos/pkg/config"
	. "github.com/kairos-io/provider-kairos/internal/cli"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

type TConfig struct {
	Kairos struct {
		NetworkToken string `yaml:"network_token"`
	} `yaml:"kairos"`
}

var _ = Describe("Get config", func() {
	Context("directory", func() {

		It("replace token in config files", func() {

			var cc string = `#node-config
kairos:
  network_token: "foo"

bb: 
  nothing: "foo"
`
			d, _ := ioutil.TempDir("", "xxxx")
			defer os.RemoveAll(d)

			err := ioutil.WriteFile(filepath.Join(d, "test"), []byte(cc), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(d, "b"), []byte(`
fooz: "bar"
			`), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			err = ReplaceToken([]string{d, "/doesnotexist"}, "baz")
			Expect(err).ToNot(HaveOccurred())

			content, err := ioutil.ReadFile(filepath.Join(d, "test"))
			Expect(err).ToNot(HaveOccurred())

			res := map[interface{}]interface{}{}

			err = yaml.Unmarshal(content, &res)
			Expect(err).ToNot(HaveOccurred())

			Expect(res).To(Equal(map[interface{}]interface{}{
				"kairos": map[interface{}]interface{}{"network_token": "baz"},
				"bb":     map[interface{}]interface{}{"nothing": "foo"},
			}))

			hasHeader, _ := config.HasHeader(string(content), "#node-config")
			Expect(hasHeader).To(BeTrue(), string(content))
		})
	})
})
