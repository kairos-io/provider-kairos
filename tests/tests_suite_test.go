package mos

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kairos-io/kairos-sdk/utils"
	process "github.com/mudler/go-processmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
	"github.com/spectrocloud/peg/pkg/machine"
	"github.com/spectrocloud/peg/pkg/machine/types"
)

var kubectl = func(s string) (string, error) {
	return Sudo("k3s kubectl " + s)
}

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kairos Test Suite")
}

func isFlavor(flavor string) bool {
	return strings.Contains(os.Getenv("FLAVOR"), flavor)
}

func screenshot() (string, error) {
	vbox, ok := Machine.(*machine.VBox)
	if ok {
		return vbox.Screenshot()
	}
	return "", fmt.Errorf("screenshot not implemented")
}
func detachAndReboot() {
	vbox, ok := Machine.(*machine.VBox)
	if ok {
		vbox.DetachCD()
		vbox.Restart()
	} else {
		Reboot()
	}
}

var sshPort string

var _ = AfterSuite(func() {
	if os.Getenv("CREATE_VM") == "true" {
		if Machine != nil {
			Machine.Stop()
			Machine.Clean()
		}
	}
	if !CurrentSpecReport().Failure.IsZero() {
		gatherLogs()
	}
})

func user() string {
	user := os.Getenv("SSH_USER")
	if user == "" {
		user = "kairos"
	}
	return user
}

func pass() string {
	pass := os.Getenv("SSH_PASS")
	if pass == "" {
		pass = "kairos"
	}

	return pass
}

var _ = BeforeSuite(func() {

	machineID := uuid.New().String()

	if os.Getenv("ISO") == "" && os.Getenv("CREATE_VM") == "true" {
		fmt.Println("ISO missing")
		os.Exit(1)
	}

	if os.Getenv("CREATE_VM") == "true" {
		t, err := ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())

		p, _ := getFreePort()
		sshPort = strconv.Itoa(p)
		if os.Getenv("SSH_PORT") != "" {
			sshPort = os.Getenv("SSH_PORT")
		}

		opts := []types.MachineOption{
			types.WithMemory("9000"),
			types.WithISO(os.Getenv("ISO")),
			types.WithSSHPort(sshPort),
			types.WithID(machineID),
			types.WithSSHUser(user()),
			types.WithSSHPass(pass()),
			types.OnFailure(func(p *process.Process) {
				out, _ := ioutil.ReadFile(p.StdoutPath())
				err, _ := ioutil.ReadFile(p.StderrPath())
				status, _ := p.ExitCode()
				fmt.Printf("VM Aborted: %s %s Exit status: %s", out, err, status)
				Fail(fmt.Sprintf("VM Aborted: %s %s Exit status: %s", out, err, status))
			}),
			types.WithStateDir(t),
			types.WithDataSource(os.Getenv("DATASOURCE")),
		}

		if os.Getenv("USE_QEMU") == "true" {
			opts = append(opts, types.QEMUEngine)
		} else {
			opts = append(opts, types.VBoxEngine)
		}

		m, err := machine.New(opts...)
		if err != nil {
			Fail(err.Error())
		}

		Machine = m

		if _, err := Machine.Create(context.Background()); err != nil {
			Fail(err.Error())
		}
	}
})

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

func gatherLogs() {
	Machine.SendFile("assets/kubernetes_logs.sh", "/tmp/logs.sh", "0770")
	Sudo("cat /oem/* > /run/oem.yaml")
	Sudo("cat /etc/resolv.conf > /run/resolv.conf")
	Sudo("k3s kubectl get pods -A -o json > /run/pods.json")
	Sudo("k3s kubectl get events -A -o json > /run/events.json")
	Sudo("cat /proc/cmdline > /run/cmdline")
	Sudo("chmod 777 /run/events.json")
	Sudo("sh /tmp/logs.sh > /run/kube_logs")
	Sudo("df -h > /run/disk")
	Sudo("mount > /run/mounts")
	Sudo("blkid > /run/blkid")
	Sudo("dmesg > /run/dmesg.log")

	GatherAllLogs(
		[]string{
			"edgevpn@kairos",
			"kairos-agent",
			"cos-setup-boot",
			"cos-setup-network",
			"cos-setup-initramfs",
			"cos-setup-reconcile",
			"kairos",
			"k3s",
			"k3s-agent",
		},
		[]string{
			"/var/log/edgevpn.log",
			"/var/log/kairos/agent.log",
			"/run/pods.json",
			"/run/disk",
			"/run/mounts",
			"/run/kube_logs",
			"/run/blkid",
			"/run/events.json",
			"/run/cmdline",
			"/run/oem.yaml",
			"/run/resolv.conf",
			"/run/dmesg.log",
		})
}

func download(s string) {
	f2, err := ioutil.TempFile("", "fff")
	Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(f2.Name())

	resp, err := http.Get(s)
	Expect(err).ToNot(HaveOccurred())

	defer resp.Body.Close()
	_, err = io.Copy(f2, resp.Body)
	Expect(err).ToNot(HaveOccurred())

	out, err := utils.SH("tar xvf " + f2.Name())
	fmt.Println(out)
	Expect(err).ToNot(HaveOccurred(), out)
}
