package role

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/kairos-io/provider-kairos/internal/assets"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
)

func generateKubeVIP(iface, ip string, args []string) (string, error) {
	out, err := utils.SH(fmt.Sprintf("kube-vip manifest daemonset --interface %s --address %s --inCluster --taint --controlplane --arp --leaderElection %s", iface, ip, strings.Join(args, " ")))

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
	if err := os.MkdirAll("/var/lib/rancher/k3s/server/manifests/", 0650); err != nil {
		return fmt.Errorf("could not create manifest dir")
	}

	targetFile := "/var/lib/rancher/k3s/server/manifests/kubevip.yaml"
	targetCRDFile := "/var/lib/rancher/k3s/server/manifests/kubevipmanifest.yaml"

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

	content, err := generateKubeVIP(iface, ip, pconfig.KubeVIP.Args)
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
