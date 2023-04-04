package main

import (
	"fmt"
	"os"

	"github.com/kairos-io/kairos-sdk/bus"
	iCli "github.com/kairos-io/provider-kairos/internal/cli"
	"github.com/kairos-io/provider-kairos/internal/provider"
	"github.com/kairos-io/provider-kairos/internal/register"

	"github.com/urfave/cli/v2"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) >= 2 && bus.IsEventDefined(os.Args[1]) {
		checkErr(provider.Start())
	}

	checkErr(Start())
}

func Start() error {
	name := "kairos-register"
	app := &cli.App{
		Name:    name,
		Version: iCli.VERSION,
		Authors: []*cli.Author{
			{
				Name: iCli.Author,
			},
		},
		Copyright:   iCli.Author,
		UsageText:   register.UsageText(name),
		Usage:       register.Usage(),
		Description: register.Description(name),
		ArgsUsage:   register.ArgsUsage(),
		Flags:       register.Flags(),
		Action:      register.Action(),
	}

	return app.Run(os.Args)
}
