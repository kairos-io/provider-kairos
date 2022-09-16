package cli

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	config "github.com/kairos-io/kairos/pkg/config"
	"github.com/kairos-io/provider-kairos/internal/provider"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	"github.com/kairos-io/provider-kairos/internal/services"
	"gopkg.in/yaml.v3"
)

func RotateToken(configDir []string, newToken, apiAddress, rootDir string, restart bool) error {
	if err := ReplaceToken(configDir, newToken); err != nil {
		return err
	}

	c, err := config.Scan(config.Directories(configDir...))
	if err != nil {
		return err
	}

	providerCfg := &providerConfig.Config{}
	err = c.Unmarshal(providerCfg)
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
	c, err := config.Scan(config.Directories(dir...))
	if err != nil {
		return fmt.Errorf("no config file found: %w", err)
	}

	header := "#node-config"

	if hasHeader, head := config.HasHeader(c.String(), ""); hasHeader {
		header = head
	}

	content := map[interface{}]interface{}{}

	if err := yaml.Unmarshal([]byte(c.String()), &content); err != nil {
		return err
	}

	section, exists := content["kairos"]
	if !exists {
		return errors.New("no kairos section in config file")
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
	content["kairos"] = piece

	d, err := yaml.Marshal(content)
	if err != nil {
		return err
	}

	fi, err := os.Stat(c.Location())
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.Location(), []byte(config.AddHeader(header, string(d))), fi.Mode().Perm())
}
