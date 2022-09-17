package role

import (
	"io/ioutil"
	"os"

	service "github.com/mudler/edgevpn/api/client/service"
)

type Role func(*service.RoleConfig) error

func SentinelExist() bool {
	if _, err := os.Stat("/usr/local/.kairos/deployed"); err == nil {
		return true
	}
	return false
}

func CreateSentinel() error {
	return ioutil.WriteFile("/usr/local/.kairos/deployed", []byte{}, os.ModePerm)
}
