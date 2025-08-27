package main

import (
	"fmt"
	"github.com/kairos-io/kairos-sdk/bus"
	"github.com/kairos-io/provider-kairos/v2/internal/cli"
	"github.com/kairos-io/provider-kairos/v2/internal/provider"
	"os"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) >= 2 && bus.IsEventDefined(os.Args[1], "init.provider.info") {
		checkErr(provider.Start())
	}

	checkErr(cli.Start())
}
