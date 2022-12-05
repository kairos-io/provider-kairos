package provider

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kairos-io/provider-kairos/internal/provider/assets"

	"github.com/kairos-io/kairos/pkg/machine"
	"github.com/kairos-io/kairos/pkg/machine/systemd"
	"github.com/kairos-io/kairos/pkg/utils"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
	"github.com/kairos-io/provider-kairos/internal/services"
	"gopkg.in/yaml.v3"

	yip "github.com/mudler/yip/pkg/schema"
)

func SaveOEMCloudConfig(name string, yc yip.YipConfig) error {
	dnsYAML, err := yaml.Marshal(yc)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join("oem", fmt.Sprintf("100_%s.yaml", name)), dnsYAML, 0700)
}

func SaveCloudConfig(name string, c []byte) error {
	return ioutil.WriteFile(filepath.Join("oem", fmt.Sprintf("%s.yaml", name)), c, 0700)
}

func SetupAPI(apiAddress, rootDir string, start bool, c *providerConfig.Config) error {
	if c.Kairos == nil || c.Kairos.NetworkToken == "" {
		return fmt.Errorf("no network token defined")
	}

	svc, err := services.P2PAPI(rootDir)
	if err != nil {
		return fmt.Errorf("could not create svc: %w", err)
	}

	apiAddress = strings.ReplaceAll(apiAddress, "https://", "")
	apiAddress = strings.ReplaceAll(apiAddress, "http://", "")

	vpnOpts := map[string]string{
		"EDGEVPNTOKEN": c.Kairos.NetworkToken,
		"APILISTEN":    apiAddress,
	}
	// Override opts with user-supplied
	for k, v := range c.VPN {
		vpnOpts[k] = v
	}

	os.MkdirAll("/etc/systemd/system.conf.d/", 0600) //nolint:errcheck
	// Setup edgevpn instance
	err = utils.WriteEnv(filepath.Join(rootDir, "/etc/systemd/system.conf.d/edgevpn-kairos.env"), vpnOpts)
	if err != nil {
		return fmt.Errorf("could not create write env file: %w", err)
	}

	err = svc.WriteUnit()
	if err != nil {
		return fmt.Errorf("could not create write unit file: %w", err)
	}

	if start {
		err = svc.Start()
		if err != nil {
			return fmt.Errorf("could not start svc: %w", err)
		}

		return svc.Enable()
	}
	return nil
}

func SetupVPN(instance, apiAddress, rootDir string, start bool, c *providerConfig.Config) error {

	if c.Kairos == nil || c.Kairos.NetworkToken == "" {
		return fmt.Errorf("no network token defined")
	}

	svc, err := services.EdgeVPN(instance, rootDir)
	if err != nil {
		return fmt.Errorf("could not create svc: %w", err)
	}

	apiAddress = strings.ReplaceAll(apiAddress, "https://", "")
	apiAddress = strings.ReplaceAll(apiAddress, "http://", "")

	vpnOpts := map[string]string{
		"EDGEVPNTOKEN": c.Kairos.NetworkToken,
		"API":          "true",
		"APILISTEN":    apiAddress,
		"DHCP":         "true",
		"DHCPLEASEDIR": "/usr/local/.kairos/lease",
	}
	// Override opts with user-supplied
	for k, v := range c.VPN {
		vpnOpts[k] = v
	}

	if c.Kairos.DNS {
		vpnOpts["DNSADDRESS"] = "127.0.0.1:53"
		vpnOpts["DNSFORWARD"] = "true"

		_ = machine.ExecuteInlineCloudConfig(assets.LocalDNS, "initramfs")
		if !utils.IsOpenRCBased() {
			svc, err := systemd.NewService(
				systemd.WithName("systemd-resolved"),
			)
			if err == nil {
				_ = svc.Restart()
			}
		}

		if err := SaveCloudConfig("vpn_dns", []byte(assets.LocalDNS)); err != nil {
			return fmt.Errorf("could not create dns config: %w", err)
		}
	}

	os.MkdirAll("/etc/systemd/system.conf.d/", 0600) //nolint:errcheck
	// Setup edgevpn instance
	err = utils.WriteEnv(filepath.Join(rootDir, "/etc/systemd/system.conf.d/edgevpn-kairos.env"), vpnOpts)
	if err != nil {
		return fmt.Errorf("could not create write env file: %w", err)
	}

	err = svc.WriteUnit()
	if err != nil {
		return fmt.Errorf("could not create write unit file: %w", err)
	}

	if start {
		err = svc.Start()
		if err != nil {
			return fmt.Errorf("could not start svc: %w", err)
		}

		return svc.Enable()
	}
	return nil
}
