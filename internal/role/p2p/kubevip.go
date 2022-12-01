package role

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/kairos-io/kairos/pkg/utils"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"
)

func generateKubeVIP(iface, ip string, args []string) (string, error) {
	out, err := utils.SH(fmt.Sprintf("kube-vip manifest daemonset --interface %s --address %s --inCluster --taint --controlplane --arp --leaderElection %s", iface, ip, strings.Join(args, " ")))

	if err != nil {
		return "", fmt.Errorf("error: %w - %s", err, out)
	}

	return out, nil
}

func downloadFromUrl(url, where string) error {
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
	os.MkdirAll("/var/lib/rancher/k3s/server/manifests/", 0650)

	targetFile := "/var/lib/rancher/k3s/server/manifests/kubevip.yaml"
	targetCRDFile := "/var/lib/rancher/k3s/server/manifests/kubevipmanifest.yaml"

	manifestUrl := "https://kube-vip.io/manifests/rbac.yaml"

	if pconfig.KubeVIP.ManifestURL != "" {
		manifestUrl = pconfig.KubeVIP.ManifestURL
	}

	err := downloadFromUrl(manifestUrl, targetCRDFile)
	if err != nil {
		return err
	}

	content, err := generateKubeVIP(iface, ip, pconfig.KubeVIP.Args)
	if err == nil {
		f, err := os.OpenFile(targetFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Println(err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + content); err != nil {
			log.Println(err)
		}
	}

	return err
}
