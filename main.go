package main

import (
	"fmt"
	"os"

	"github.com/kairos-io/kairos/sdk/bus"
	cli "github.com/kairos-io/provider-kairos/internal/cli"

	"github.com/kairos-io/provider-kairos/internal/provider"
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
