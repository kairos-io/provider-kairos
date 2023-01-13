package cli

import (
	"context"
	"fmt"
	"io/ioutil" // nolint
	"os"

	nodepair "github.com/mudler/go-nodepair"
	qr "github.com/mudler/go-nodepair/qrcode"
)

// isDirectory determines if a file represented
// by `path` is a directory or not.
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}

func isReadable(fileName string) bool {
	file, err := os.Open(fileName)
	if err != nil {
		if os.IsPermission(err) {
			return false
		}
	}
	file.Close()
	return true
}

func register(loglevel, arg, configFile, device string, reboot, poweroff bool) error {
	b, _ := ioutil.ReadFile(configFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if arg != "" {
		isDir, err := isDirectory(arg)
		if err == nil && isDir {
			return fmt.Errorf("Cannot register with a directory, please pass a file.") //nolint:revive // This is a message printed to the user.
		} else if err != nil {
			return err
		}
		if !isReadable(arg) {
			return fmt.Errorf("Cannot register with a file that is not readable.") //nolint:revive // This is a message printed to the user.
		}
	}
	// dmesg -D to suppress tty ev
	fmt.Println("Sending registration payload, please wait")

	config := map[string]string{
		"device": device,
		"cc":     string(b),
	}

	if reboot {
		config["reboot"] = ""
	}

	if poweroff {
		config["poweroff"] = ""
	}

	err := nodepair.Send(
		ctx,
		config,
		nodepair.WithReader(qr.Reader),
		nodepair.WithToken(arg),
		nodepair.WithLogLevel(loglevel),
	)
	if err != nil {
		return err
	}

	fmt.Println("Payload sent, installation will start on the machine briefly")
	return nil
}
