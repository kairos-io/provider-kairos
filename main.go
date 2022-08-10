package main

import (
	"fmt"
	"os"

	"github.com/c3os-io/c3os/sdk/bus"

	"github.com/c3os-io/provider-c3os/internal/provider"
	"github.com/mudler/go-pluggable"
)

func main() {
	factory := pluggable.NewPluginFactory()

	if len(os.Args) <= 2 {
		fmt.Println("Invalid arguments")
		os.Exit(1)
	}

	// TODO:
	// if os.Args[1] not in sdk.Events
	// kick the CLI
	// otherwise kick the provider plugin

	// then symlink this as the c3os-cli.

	// Input: bus.EventInstallPayload
	// Expected output: map[string]string{}
	factory.Add(bus.EventInstall, provider.Install)

	factory.Add(bus.EventBootstrap, provider.Bootstrap)

	// Input: config
	// Expected output: string
	factory.Add(bus.EventChallenge, provider.Challenge)

	err := factory.Run(pluggable.EventType(os.Args[1]), os.Stdin, os.Stdout)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
