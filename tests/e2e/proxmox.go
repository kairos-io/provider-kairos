package mos

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/gomega"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/kairos-io/kairos-sdk/utils"
	"golang.org/x/crypto/ssh"

	"github.com/luthermonson/go-proxmox"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func datasourceISO(cc []byte, output string) error {
	temp, err := os.MkdirTemp("", "datasource")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	os.WriteFile(filepath.Join(temp, "meta-data"), []byte{}, os.ModePerm)

	os.WriteFile(filepath.Join(temp, "user-data"), cc, os.ModePerm)

	out, err := utils.SH(fmt.Sprintf("cd %s && mkisofs -output %s -volid cidata -joliet -rock user-data meta-data", temp, output))
	if err != nil {
		return fmt.Errorf("failed %s: %w", out, err)
	}

	return nil
}

// use as:
// 	node, err := getNode()

// 	storage, err := node.Storage("local")
// 	if err != nil {
// 		panic(err)
// 	}

// 	uploadCloudInitISO(
// 		"foo.iso",
// 		[]byte(`#cloud-config
// install:
//   auto: true
//   device: "auto"
//   reboot: true

// k3s:
//   enable: true

//	`), storage,
//
// )
func uploadCloudInitISO(isoname string, cc []byte, storage *proxmox.Storage) error {

	temp, err := os.MkdirTemp("", "datasource")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	if err := datasourceISO(cc, filepath.Join(temp, isoname)); err != nil {
		return err
	}

	tup, err := storage.Upload("iso", filepath.Join(temp, isoname))
	return tup.WaitFor(300)
}

func NewSCPClient(user, pass, host string) scp.Client {
	sshConfig := sshConfig(user, pass)

	return scp.NewClientWithTimeout(host, sshConfig, 10*time.Second)
}
func SSHCommand(user, pass, host, cmd string) (string, error) {
	client, session, err := NewClient(user, pass, host)
	if err != nil {
		return "", err
	}
	defer client.Close()
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), err
	}

	return string(out), err
}

// NewClient returns a new ssh client associated to a machine
func NewClient(user, pass, host string) (*ssh.Client, *ssh.Session, error) {
	sshConfig := sshConfig(user, pass)

	client, err := SSHDialTimeout("tcp", host, sshConfig, 30*time.Second)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}

func sshConfig(user, pass string) *ssh.ClientConfig {
	sshConfig := &ssh.ClientConfig{
		User:    user,
		Auth:    []ssh.AuthMethod{ssh.Password(pass)},
		Timeout: 30 * time.Second, // max time to establish connection
	}

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	return sshConfig
}

func getNode() (*proxmox.Node, *proxmox.Client, error) {
	insecureHTTPClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	client := proxmox.NewClient(os.Getenv("PROXMOX_ENDPOINT"),
		proxmox.WithClient(&insecureHTTPClient),
		//proxmox.WithAPIToken(tokenID, secret),
		proxmox.WithLogins(os.Getenv("PROXMOX_USER"), os.Getenv("PROXMOX_PASS")),
	)

	version, err := client.Version()
	if err != nil {
		panic(err)
	}
	fmt.Println(version.Release) // 6.3

	statuses, err := client.Nodes()

	for _, st := range statuses {
		fmt.Println(st.Node)
	}

	node, err := client.Node(os.Getenv("PROXMOX_NODE"))
	return node, client, err
}

func EventuallyConnects(user, pass, host string, t ...int) {
	dur := 360
	if len(t) > 0 {
		dur = t[0]
	}
	EventuallyWithOffset(1, func() string {
		out, err := SSHCommand(user, pass, host, "echo ping")
		if err != nil {
			fmt.Println(err)
		}
		return out
	}, time.Duration(time.Duration(dur)*time.Second), time.Duration(30*time.Second)).Should(Equal("ping\n"))
}

// pixiecore:
// docker run -d --name pixiecore --net=host -v $PWD:/files quay.io/pixiecore/pixiecore boot /files/kairos-opensuse-${VERSION}-kernel /files/kairos-opensuse-${VERSION}-initrd --cmdline="rd.neednet=1 ip=dhcp rd.cos.disable root=live:{{ ID \"/files/kairos-opensuse-${VERSION}.squashfs\" }} netboot nodepair.enable config_url={{ ID \"/files/config.yaml\" }} console=tty1 console=ttyS0 console=tty0"
func stopVPN(ControlVM *SSHConn) {
	out, err := ControlVM.Command("sudo /bin/bash -c 'systemctl stop vpn && rm -rf /etc/systemd/system/vpn.service && systemctl daemon-reload && rm -rf /usr/local/vpn.sh'")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	fmt.Println(out)
}

func startVPN(networkToken string, ControlVM *SSHConn) {
	//
	out, err := ControlVM.Command("sudo modprobe tun")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	// NOTE: This requires systemd
	// Get the controlVM on the same VPN so it can reach the cluster
	out, err = ControlVM.Command(fmt.Sprintf(`cat << EOF > /tmp/vpn.sh
#!/bin/bash
EDGEVPNTOKEN=%s sudo -E edgevpn --log-level debug
EOF`, networkToken))
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = ControlVM.Command("sudo mv /tmp/vpn.sh /usr/local/vpn.sh && sudo chmod +x /usr/local/vpn.sh")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = ControlVM.Command(`cat << EOF > /tmp/vpn.service
[Unit]
Description=vpn
[Service]
Type=simple
Restart=always
RestartSec=1
ExecStart=/usr/local/vpn.sh

[Install]
WantedBy=multi-user.target
EOF`)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = ControlVM.Command("sudo /bin/bash -c 'mv /tmp/vpn.service /etc/systemd/system/vpn.service && systemctl daemon-reload && systemctl start vpn'")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	fmt.Println(out)

}

func ping(ip string) {
	EventuallyWithOffset(1, func() string {
		out, err := ControlVM.Command(fmt.Sprintf("ping %s -c 3", ip))
		if err != nil {
			fmt.Println(err)
		}
		return out
	}, time.Duration(time.Duration(650)*time.Second), time.Duration(30*time.Second)).Should(ContainSubstring("3 received"))
}
