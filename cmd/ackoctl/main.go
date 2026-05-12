package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/cli"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(cli.BuildInfo{Version: version, Commit: commit, Date: date})

	// Bind ctrl-c / SIGTERM to a cancelable context so long-running scans and
	// queries can be aborted instead of waiting on the HTTP timeout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		printError(err)
		os.Exit(1)
	}
}

// printError gives users a single line of actionable context when something
// went wrong. APIError already carries the FastAPI detail; config errors get
// a "hint" line pointing at the config command, and signal cancellation is
// surfaced explicitly so the user knows the run was aborted rather than
// silently truncated.
func printError(err error) {
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(os.Stderr, "Error: aborted")
	case errors.Is(err, config.ErrNoCurrent),
		errors.Is(err, config.ErrNoContext),
		errors.Is(err, config.ErrContextNotFound):
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprintln(os.Stderr, "hint: run `ackoctl config set-context <name> --server <url>` and `ackoctl config use-context <name>`")
	default:
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			fmt.Fprintln(os.Stderr, "Error:", apiErr.Error())
			return
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
}
