package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/cli"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() { os.Exit(run()) }

// run is the real entry point. main() only forwards the exit code so that
// `os.Exit` does not skip the signal-handler cleanup `defer`s.
func run() int {
	cli.SetBuildInfo(cli.BuildInfo{Version: version, Commit: commit, Date: date})

	// Bind ctrl-c / SIGTERM to a cancelable context so long-running scans and
	// queries can be aborted instead of waiting on the HTTP timeout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCmd()
	err := root.ExecuteContext(ctx)

	// Give the background version-check a short grace period to flush its
	// cache so first-time users build up a cache after one quick command.
	// Capped tightly so an offline run never feels slow.
	cli.WaitForBackgroundChecks(200 * time.Millisecond)

	if err != nil {
		printError(err)
		return 1
	}
	return 0
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
