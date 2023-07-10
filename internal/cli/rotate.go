package cli

import (
	"errors"
	"fmt"
	"io/ioutil" // nolint
	"os"
	"path/filepath"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	"github.com/kairos-io/kairos-sdk/collector"
	"github.com/kairos-io/kairos-sdk/unstructured"
	"github.com/kairos-io/provider-kairos/v2/internal/provider"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	"github.com/kairos-io/provider-kairos/v2/internal/services"
	"gopkg.in/yaml.v3"
)

func RotateToken(configDir []string, newToken, apiAddress, rootDir string, restart bool) error {
	if err := ReplaceToken(configDir, newToken); err != nil {
		return err
	}

	o := &collector.Options{}
	if err := o.Apply(collector.Directories(configDir...)); err != nil {
		return err
	}
	c, err := collector.Scan(o, config.FilterKeys)

	if err != nil {
		return err
	}

	providerCfg := &providerConfig.Config{}
	a, _ := c.String()
	err = yaml.Unmarshal([]byte(a), providerCfg)
	if err != nil {
		return err
	}

	err = provider.SetupVPN(services.EdgeVPNDefaultInstance, apiAddress, rootDir, false, providerCfg)
	if err != nil {
		return err
	}

	if restart {
		svc, err := services.EdgeVPN(services.EdgeVPNDefaultInstance, rootDir)
		if err != nil {
			return err
		}

		return svc.Restart()
	}
	return nil
}

func ReplaceToken(dir []string, token string) (err error) {
	locations, err := FindYAMLWithKey("p2p.network_token", collector.Directories(dir...))
	if err != nil {
		return err
	}
	for _, f := range locations {
		dat, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("warning: could not read %s '%s'\n", f, err.Error())
		}

		header := config.DefaultHeader
		if hasHeader, head := config.HasHeader(string(dat), ""); hasHeader {
			header = head
		}
		content := map[interface{}]interface{}{}

		if err := yaml.Unmarshal(dat, &content); err != nil {
			return err
		}

		section, exists := content["p2p"]
		if !exists {
			return errors.New("no p2p section in config file")
		}

		dd, err := yaml.Marshal(section)
		if err != nil {
			return err
		}

		piece := map[string]interface{}{}

		if err := yaml.Unmarshal(dd, &piece); err != nil {
			return err
		}

		piece["network_token"] = token
		content["p2p"] = piece

		d, err := yaml.Marshal(content)
		if err != nil {
			return err
		}

		fi, err := os.Stat(f)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(f, []byte(config.AddHeader(header, string(d))), fi.Mode().Perm()); err != nil {
			return err
		}
	}

	return nil
}

// FindYAMLWithKey will find and return files that contain a given key in them.
func FindYAMLWithKey(s string, opts ...collector.Option) ([]string, error) {
	o := &collector.Options{}

	var result []string
	if err := o.Apply(opts...); err != nil {
		return result, err
	}

	files := allFiles(o.ScanDir)

	for _, f := range files {
		dat, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("warning: skipping file '%s' - %s\n", f, err.Error())
		}

		found, err := unstructured.YAMLHasKey(s, dat)
		if err != nil {
			fmt.Printf("warning: skipping file '%s' - %s\n", f, err.Error())
		}

		if found {
			result = append(result, f)
		}

	}

	return result, nil
}

func allFiles(dir []string) []string {
	var files []string
	for _, d := range dir {
		if f, err := listFiles(d); err == nil {
			files = append(files, f...)
		}
	}
	return files
}

func listFiles(dir string) ([]string, error) {
	var content []string

	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				content = append(content, path)
			}

			return nil
		})

	return content, err
}
