package mos_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kairos-io/kairos/pkg/utils"
	"github.com/luthermonson/go-proxmox"
	process "github.com/mudler/go-processmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kairos E2E proxmox Test Suite")
}

func genToken() (string, error) {
	return utils.SH("/usr/bin/edgevpn -b -g")
}

var ControlVM *SSHConn

func waitTask(t *proxmox.Task, err error) {
	Expect(err).ToNot(HaveOccurred())
	Expect(t).ToNot(BeNil())
	err = t.Wait(1*time.Second, 10*time.Second)
	Expect(err).ToNot(HaveOccurred())
}

func deleteVMs(node *proxmox.Node) {
	fmt.Println("delete all")
	vms, err := node.VirtualMachines()
	if err != nil {
		Fail(err.Error())
	}

	// Delete all VMs running
	for _, vm := range vms {
		v, _ := node.VirtualMachine(int(vm.VMID))
		fmt.Println(v)
		d, _ := json.Marshal(v)
		fmt.Println(string(d))
		fmt.Printf("Deleting %s\n", vm.Name)

		t, err := vm.Stop()
		waitTask(t, err)

		t, err = vm.Delete()
		waitTask(t, err)
	}
}

var _ = AfterSuite(func() {
	node, _, _ := getNode()
	deleteVMs(node)
})

// accessors
func proxmoxISO() string {
	return os.Getenv("PROXMOX_ISO")
}

func isoName() string {
	return filepath.Base(proxmoxISO())
}

func locateISOFmt(s string) string {
	return fmt.Sprintf("local:iso/%s,media=cdrom", s)
}

var _ = BeforeSuite(func() {

	// We need to setup the router VM where we will connect to
	// First upload the ISOs needed (OS and cidata)
	node, client, err := getNode()
	if err != nil {
		Fail(err.Error())
	}
	storage, err := node.Storage(os.Getenv("PROXMOX_STORAGE"))
	if err != nil {
		Fail(err.Error())
	}

	temp, err := os.MkdirTemp("", "datasource")
	if err != nil {
		Fail(err.Error())
	}
	defer os.RemoveAll(temp)

	token, err := genToken()
	if err != nil {
		Fail(err.Error() + token)
	}
	fmt.Println("start vpn")

	fmt.Println("Upload cloud init")
	err = uploadCloudInitISO(
		"control.iso",
		[]byte(fmt.Sprintf(`#cloud-config

stages:
  boot.after:
  - commands:
    - EDGEVPNTOKEN=%s edgevpn service-add "ssh" "127.0.0.1:22"
users:
- name: "kairos"
  passwd: "kairos"

install:
  auto: true
  device: "auto"
  reboot: true
k3s:
  enable: true`, token)), storage,
	)
	if err != nil {
		Fail(err.Error())
	}

	iso, err := storage.ISO(isoName())
	if err != nil || iso == nil {
		fmt.Println("Upload target iso")
		tup, err := storage.Upload("iso", proxmoxISO())
		if err != nil {
			Fail(err.Error())
		}
		if err := tup.WaitFor(300); err != nil {
			Fail(err.Error())
		}
	}

	deleteVMs(node)

	// Create control VM and wait for it to be available
	cluster, err := client.Cluster()
	Expect(err).ToNot(HaveOccurred())

	nextid, err := cluster.NextID()
	Expect(err).ToNot(HaveOccurred())

	fmt.Println("Next ID", nextid)

	// See: https://pve.proxmox.com/pve-docs/api-viewer/index.html#/nodes/{node}/qemu
	t, err := node.NewVirtualMachine(nextid,
		proxmox.VirtualMachineOption{Name: "serial0", Value: "socket"},
		proxmox.VirtualMachineOption{Name: "memory", Value: "2048"},
		proxmox.VirtualMachineOption{Name: "cores", Value: "1"},
		proxmox.VirtualMachineOption{Name: "boot", Value: "order=scsi0;ide0;net0"},
		proxmox.VirtualMachineOption{Name: "net0", Value: "virtio,bridge=vmbr0,firewall=1"},
		proxmox.VirtualMachineOption{Name: "scsi0", Value: "local-lvm:60,size=60G"},
		proxmox.VirtualMachineOption{Name: "ide0", Value: locateISOFmt(isoName())},
		proxmox.VirtualMachineOption{Name: "ide1", Value: locateISOFmt("control.iso")},
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(t).ToNot(BeNil())
	err = t.Wait(1*time.Second, 10*time.Second)
	Expect(err).ToNot(HaveOccurred())

	v, _ := node.VirtualMachine(nextid)
	v.Start()

	edgevpn := process.New(
		process.WithEnvironment(fmt.Sprintf("EDGEVPNTOKEN=%s", token)),
		process.WithName("/usr/bin/edgevpn"), process.WithArgs("service-connect", "--log-level", "debug", "ssh", "0.0.0.0:9090"), process.WithStateDir(temp))
	err = edgevpn.Run()

	// fmt.Println("start vpn", err)

	//defer edgevpn.Stop()
	ControlVM = NewSSH("kairos", "kairos", "127.0.0.1:9090")

	Eventually(func() string {
		out, err := ControlVM.Command("kairos-agent state get boot")
		if err != nil {
			fmt.Println(err)
		}
		return out
	}, time.Duration(time.Duration(920)*time.Second), time.Duration(30*time.Second)).Should(Equal("active_boot"))

	out, err := ControlVM.Command(`ssh-keygen -t rsa -q -f "$HOME/.ssh/id_rsa" -N ""`)
	Expect(err).ToNot(HaveOccurred(), out)
	fmt.Println(out)
})
