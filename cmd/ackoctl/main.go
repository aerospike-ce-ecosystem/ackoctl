package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
		return printError(os.Stderr, err)
	}
	return 0
}

// exitAborted is the exit code returned when the run is cancelled by a signal
// (ctrl-c / SIGTERM). 130 == 128 + SIGINT is the shell convention, so callers
// like `set -e` scripts and CI runners can tell a user abort apart from a real
// error (which keeps the generic exit code 1).
const exitAborted = 130

// printError gives users a single line of actionable context when something
// went wrong and returns the process exit code. APIError already carries the
// FastAPI detail; config errors get a "hint" line pointing at the config
// command, and signal cancellation is surfaced explicitly so the user knows
// the run was aborted rather than silently truncated.
func printError(w io.Writer, err error) int {
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(w, "Error: aborted")
		return exitAborted
	case errors.Is(err, config.ErrNoCurrent),
		errors.Is(err, config.ErrNoContext),
		errors.Is(err, config.ErrContextNotFound):
		fmt.Fprintln(w, "Error:", err)
		fmt.Fprintln(w, "hint: run `ackoctl config set-context <name> --server <url>` and `ackoctl config use-context <name>`")
		return 1
	default:
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			fmt.Fprintln(w, "Error:", apiErr.Error())
			return 1
		}
		fmt.Fprintln(w, "Error:", err)
		return 1
	}
}
