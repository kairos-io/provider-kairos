package role

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/kairos-io/provider-kairos/v2/internal/assets"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
	kubeVipCmd "github.com/kube-vip/kube-vip/cmd"
	"github.com/kube-vip/kube-vip/pkg/kubevip"
)

var (
	initConfig       kubevip.Config
	initLoadBalancer kubevip.LoadBalancer
)

// Generates the kube-vip manifest based on the command type
func generateKubeVIPv2(command string, iface, ip string, kConfig *providerConfig.Config) (string, error) {
	// Comand can be "manifest" or "daemonset"
	// iface is the interface name
	// ip is the VIP address
	var err error

	// Set the kube-vip config based on the provider config and what we loaded from config files
	applyKConfigToInitConfig(kConfig.KubeVIP, &initConfig)

	// Now set the values coming from env vars
	if err := kubevip.ParseEnvironment(&initConfig); err != nil {
		return "", fmt.Errorf("parsing environment: %w", err)
	}

	// Now the manual ones that are hardcoded by us
	initConfig.Interface = iface
	initConfig.Address = ip
	initConfig.EnableControlPlane = true
	initConfig.EnableARP = true
	initConfig.EnableLeaderElection = true
	initConfig.LoadBalancers = append(initConfig.LoadBalancers, initLoadBalancer)

	// The control plane has a requirement for a VIP being specified
	if initConfig.EnableControlPlane && (initConfig.VIP == "" && initConfig.Address == "" && !initConfig.DDNS) {
		return "", fmt.Errorf("no address is specified for kube-vip to expose services on")
	}

	// Ensure there is an address to generate the CIDR from
	if initConfig.VIPSubnet == "" && initConfig.Address != "" {
		initConfig.VIPSubnet, err = kubeVipCmd.GenerateCidrRange(initConfig.Address)
		if err != nil {
			return "", fmt.Errorf("config parse: %w", err)
		}
	}

	switch strings.ToLower(command) {
	case "daemonset":
		return kubevip.GenerateDaemonsetManifestFromConfig(&initConfig, kubeVipCmd.Release.Version, true, true), nil
	case "pod":
		return kubevip.GeneratePodManifestFromConfig(&initConfig, kubeVipCmd.Release.Version, true), nil
	}
	return "", fmt.Errorf("unknown manifest type %s", command)
}

// applyKConfigToInitConfig applies the KubeVIP configuration to the initConfig
// by iterating over the fields of the KubeVIP struct and setting the corresponding
// fields in the initConfig struct. It uses reflection to access the fields dynamically.
// This allows us to replicate the kubevip.Config struct in our provider config directly
func applyKConfigToInitConfig(kConfig providerConfig.KubeVIP, initConfig *kubevip.Config) {
	kConfigValue := reflect.ValueOf(kConfig)
	kConfigType := reflect.TypeOf(kConfig)
	initConfigValue := reflect.ValueOf(initConfig).Elem()

	for i := 0; i < kConfigType.NumField(); i++ {
		kField := kConfigType.Field(i)
		kValue := kConfigValue.Field(i)

		// Check if the field exists in initConfig
		initField := initConfigValue.FieldByName(kField.Name)
		if initField.IsValid() && initField.Type() == kField.Type {
			// Set the value from kConfig to initConfig
			initField.Set(kValue)
		}
	}
}

func downloadFromURL(url, where string) error {
	output, err := os.Create(where)
	if err != nil {
		return err
	}
	defer output.Close()

	response, err := http.Get(url)
	if err != nil {
		return err

	}
	defer response.Body.Close()

	_, err = io.Copy(output, response.Body)
	return err
}

func deployKubeVIP(iface, ip string, pconfig *providerConfig.Config) error {
	manifestDirectory := "/var/lib/rancher/k3s/server/manifests/"
	if pconfig.K3sAgent.Enabled {
		manifestDirectory = "/var/lib/rancher/k3s/agent/pod-manifests/"
	}
	if err := os.MkdirAll(manifestDirectory, 0650); err != nil {
		return fmt.Errorf("could not create manifest dir")
	}

	targetFile := manifestDirectory + "kubevip.yaml"
	targetCRDFile := manifestDirectory + "kubevipmanifest.yaml"

	command := "daemonset"
	if pconfig.KubeVIP.StaticPod {
		command = "pod"
	}

	if pconfig.KubeVIP.ManifestURL != "" {
		err := downloadFromURL(pconfig.KubeVIP.ManifestURL, targetCRDFile)
		if err != nil {
			return err
		}
	} else {
		f, err := assets.GetStaticFS().Open("kube_vip_rbac.yaml")
		if err != nil {
			return fmt.Errorf("could not find kube_vip in assets")
		}
		defer f.Close()

		destination, err := os.Create(targetCRDFile)
		if err != nil {
			return err
		}
		defer destination.Close()
		_, err = io.Copy(destination, f)
		if err != nil {
			return err
		}
	}

	content, err := generateKubeVIPv2(command, iface, ip, pconfig)
	if err != nil {
		return fmt.Errorf("could not generate kubevip %s", err.Error())
	}
	fmt.Println(content)

	f, err := os.Create(targetFile)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", f.Name(), err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("could not write to %s: %w", f.Name(), err)
	}

	return nil
}
