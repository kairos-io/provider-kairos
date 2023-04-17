package register

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	nodepair "github.com/mudler/go-nodepair"
	qr "github.com/mudler/go-nodepair/qrcode"
)

// Flags is only used temporarily to avoid duplication while the register sub-command is deprecated.
func Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "config",
			Usage:    "Kairos YAML configuration file",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "device",
			Usage: "Device used for the installation target",
		},
		&cli.BoolFlag{
			Name:  "reboot",
			Usage: "Reboot node after installation",
		},
		&cli.BoolFlag{
			Name:  "poweroff",
			Usage: "Shutdown node after installation",
		},
		&cli.StringFlag{
			Name:  "log-level",
			Usage: "Set log level",
		},
	}
}

// Action is only used temporarily to avoid duplication while the register sub-command is deprecated.
func Action() func(c *cli.Context) error {
	return func(c *cli.Context) error {
		var ref string
		if c.Args().Len() == 1 {
			ref = c.Args().First()
		}

		return Register(c.String("log-level"), ref, c.String("config"), c.String("device"), c.Bool("reboot"), c.Bool("poweroff"))
	}
}

// UsageText is only used temporarily to avoid duplication while the register sub-command is deprecated.
func UsageText(name string) string {
	return name + " --reboot --device /dev/sda /image/snapshot.png"
}

// Usage is only used temporarily to avoid duplication while the register sub-command is deprecated.
func Usage() string {
	return "Registers and bootstraps a node"
}

// ArgsUsage is only used temporarily to avoid duplication while the register sub-command is deprecated.
func ArgsUsage() string {
	return "Register optionally accepts an image. If nothing is passed will take a screenshot of the screen and try to decode the QR code"
}

// Description is only used temporarily to avoid duplication while the register sub-command is deprecated.
func Description(name string) string {
	return fmt.Sprintf(` 
		Bootstraps a node which is started in pairing mode. It can send over a configuration file used to install the kairos node.

		For example:
		$ %s --config config.yaml --device /dev/sda ~/Downloads/screenshot.png

		will decode the QR code from ~/Downloads/screenshot.png and bootstrap the node remotely.

		If the image is omitted, a screenshot will be taken and used to decode the QR code.

		See also https://kairos.io/docs/getting-started/ for documentation.
		`, name)
}

// Command is only used temporarily to avoid duplication while the kairosctl sub-command is deprecated.
func Command(warn bool) *cli.Command {
	subCommandName := "register"
	fullName := "kairos " + subCommandName
	usage := Usage()
	description := Description(fullName)
	if warn {
		usage += " (WARNING: this command will be deprecated in the next release, use the kairosctl binary instead)"
		description = "\t\tWARNING: This command will be deprecated in the next release. Please use the new kairosctl binary to register your nodes.\n" + description
	}

	var command = cli.Command{
		Name:        subCommandName,
		UsageText:   UsageText(fullName),
		Usage:       usage,
		Description: description,
		ArgsUsage:   ArgsUsage(),
		Flags:       Flags(),
		Action:      Action(),
	}

	return &command
}

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

// Register is only exposed while the register sub-command is deprecated.
func Register(loglevel, arg, configFile, device string, reboot, poweroff bool) error {
	b, _ := os.ReadFile(configFile)
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
