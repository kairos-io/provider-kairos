package provider

import (
	"os"

	"github.com/c3os-io/c3os/sdk/bus"

	"github.com/mudler/go-pluggable"
)

func Start() error {
	factory := pluggable.NewPluginFactory()

	// Input: bus.EventInstallPayload
	// Expected output: map[string]string{}
	factory.Add(bus.EventInstall, Install)

	factory.Add(bus.EventBootstrap, Bootstrap)

	// Input: config
	// Expected output: string
	factory.Add(bus.EventChallenge, Challenge)

	return factory.Run(pluggable.EventType(os.Args[1]), os.Stdin, os.Stdout)

}
