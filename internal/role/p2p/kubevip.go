package role

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/kairos-io/provider-kairos/v2/internal/assets"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"
)

func generateKubeVIP(command string, iface, ip string, args []string) (string, error) {
	out, err := utils.SH(fmt.Sprintf("kube-vip manifest %s --interface %s --address %s --inCluster --taint --controlplane --arp --leaderElection %s", command, iface, ip, strings.Join(args, " ")))

	if err != nil {
		return "", fmt.Errorf("error: %w - %s", err, out)
	}

	return out, nil
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

	content, err := generateKubeVIP(command, iface, ip, pconfig.KubeVIP.Args)
	if err != nil {
		return fmt.Errorf("could not generate kubevip %s", err.Error())
	}

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
