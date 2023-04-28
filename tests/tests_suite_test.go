package mos

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
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

var kubectl = func(vm VM, s string) (string, error) {
	return vm.Sudo("k3s kubectl " + s)
}

var getVersionCmd = ". /etc/os-release; [ ! -z \"$KAIROS_VERSION\" ] && echo $KAIROS_VERSION || echo $VERSION"

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kairos Test Suite")
}

func isFlavor(vm VM, flavor string) bool {
	out, err := vm.Sudo(fmt.Sprintf("cat /etc/os-release | grep ID=%s", flavor))
	return err == nil && out != ""
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

func startVM(iso string) (context.Context, VM) {
	var sshPort, spicePort int

	vmName := uuid.New().String()

	stateDir, err := os.MkdirTemp("", "stateDir-*")
	Expect(err).ToNot(HaveOccurred())

	sshPort, err = getFreePort()
	Expect(err).ToNot(HaveOccurred())

	memory := os.Getenv("MEMORY")
	if memory == "" {
		memory = "2000"
	}
	cpus := os.Getenv("CPUS")
	if cpus == "" {
		cpus = "1"
	}

	opts := []types.MachineOption{
		types.QEMUEngine,
		types.WithISO(iso),
		types.WithMemory(memory),
		types.WithDriveSize("25000"),
		types.WithCPU(cpus),
		types.WithSSHPort(strconv.Itoa(sshPort)),
		types.WithID(vmName),
		types.WithSSHUser(user()),
		types.WithSSHPass(pass()),
		types.OnFailure(func(p *process.Process) {
			out, _ := os.ReadFile(p.StdoutPath())
			err, _ := os.ReadFile(p.StderrPath())
			status, _ := p.ExitCode()

			// We are explicitly killing the qemu process. We don't treat that as an error
			// but we just print the output just in case.
			fmt.Printf("\nVM Aborted: %s %s Exit status: %s\n", out, err, status)
		}),
		types.WithStateDir(stateDir),
		types.WithDataSource(os.Getenv("DATASOURCE")),
	}
	if os.Getenv("KVM") != "" {
		opts = append(opts, func(m *types.MachineConfig) error {
			m.Args = append(m.Args,
				"-enable-kvm",
			)
			return nil
		})
	}

	if os.Getenv("USE_QEMU") == "true" {
		opts = append(opts, types.QEMUEngine)

		// You can connect to it with "spicy" or other tool.
		// DISPLAY is already taken on Linux X sessions
		if os.Getenv("MACHINE_SPICY") != "" {
			spicePort, _ = getFreePort()
			for spicePort == sshPort { // avoid collision
				spicePort, _ = getFreePort()
			}
			display := fmt.Sprintf("-spice port=%d,addr=127.0.0.1,disable-ticketing=yes", spicePort)
			opts = append(opts, types.WithDisplay(display))

			cmd := exec.Command("spicy",
				"-h", "127.0.0.1",
				"-p", strconv.Itoa(spicePort))
			err = cmd.Start()
			Expect(err).ToNot(HaveOccurred())
		}
	} else {
		opts = append(opts, types.VBoxEngine)
	}
	m, err := machine.New(opts...)
	Expect(err).ToNot(HaveOccurred())

	vm := NewVM(m, stateDir)

	ctx, err := vm.Start(context.Background())
	if err != nil {
		so, e := os.ReadFile(path.Join(stateDir, "stdout"))
		if e != nil {
			fmt.Printf("Error reading stdout after process failing %s\n", e.Error())
		}
		se, e := os.ReadFile(path.Join(stateDir, "stderr"))
		if e != nil {
			fmt.Printf("Error reading stderr after process failing %s\n", e.Error())
		}
		fmt.Printf("An error occured.\nStderr = %+v\nStdout = %+v\n", string(se), string(so))
	}
	Expect(err).ToNot(HaveOccurred())

	return ctx, vm
}

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

func gatherLogs(vm VM) {
	vm.Scp("assets/kubernetes_logs.sh", "/tmp/logs.sh", "0770")
	vm.Sudo("cat /oem/* > /run/oem.yaml")
	vm.Sudo("cat /etc/resolv.conf > /run/resolv.conf")
	vm.Sudo("k3s kubectl get pods -A -o json > /run/pods.json")
	vm.Sudo("k3s kubectl get events -A -o json > /run/events.json")
	vm.Sudo("cat /proc/cmdline > /run/cmdline")
	vm.Sudo("chmod 777 /run/events.json")
	vm.Sudo("sh /tmp/logs.sh > /run/kube_logs")
	vm.Sudo("df -h > /run/disk")
	vm.Sudo("mount > /run/mounts")
	vm.Sudo("blkid > /run/blkid")
	vm.Sudo("dmesg > /run/dmesg.log")

	vm.GatherAllLogs(
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

// kairosCli can be used to issue commands to the kairos-provider cli as if
// it was compiled and put in the PATH. This is running the CLI using `go run`
// to ensure we are running the same code that is being tested (and not some
// previously compiled binary).
// This makes the tests self-contained so that they don't rely on previous steps
// to have been run.
func kairosCli(cmd string) (string, error) {
	return utils.SH(fmt.Sprintf("go run ../main.go -- %s", cmd))
}

func kairosCtlCli(cmd string) (string, error) {
	return utils.SH(fmt.Sprintf("go run ../cli/kairosctl/main.go -- %s", cmd))
}
