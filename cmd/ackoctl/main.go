package main

import (
	"fmt"
	"os"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
