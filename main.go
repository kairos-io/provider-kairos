package main

import (
	"fmt"
	"os"

	"github.com/c3os-io/c3os/sdk/bus"
	cli "github.com/c3os-io/provider-c3os/internal/cli"

	"github.com/c3os-io/provider-c3os/internal/provider"
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

	checkErr(cli.Start())
}
