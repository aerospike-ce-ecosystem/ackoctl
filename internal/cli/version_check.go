package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/release"
)

// envOptOut is honoured in addition to --no-version-check so users can flip
// the check off globally without touching every CLI invocation.
const envOptOut = "ACKOCTL_NO_VERSION_CHECK"

// refreshTimeout caps the background tag lookup. Short — the check is best-
// effort, and if GitHub is slow we'd rather skip than slow down `ackoctl exit`.
const refreshTimeout = 1500 * time.Millisecond

// refreshWG tracks the background cache refresh goroutine so main can give
// it a short grace period to finish before os.Exit terminates the process.
// Without this, fast commands ("ackoctl config view") exit before the
// goroutine ever writes to disk, and the cache never warms up.
var refreshWG sync.WaitGroup

// WaitForBackgroundChecks blocks until the background version-check
// goroutine finishes, or until the provided timeout elapses. Called from
// main() after Cobra returns.
func WaitForBackgroundChecks(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		refreshWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// runVersionCheck is invoked from the root command's PersistentPreRunE. It:
//   - reads the on-disk cache; if fresh + outdated, prints a one-line warning;
//   - fires a detached goroutine to refresh the cache for the next run.
//
// The function never returns an error to the command runner — the check is
// strictly advisory.
func runVersionCheck(cmd *cobra.Command, optOut bool) {
	if optOut || os.Getenv(envOptOut) == "1" {
		return
	}
	if shouldSkipVersionCheck(cmd) {
		return
	}
	if release.IsDevBuild(buildInfo.Version) {
		return
	}

	cachePath, err := release.DefaultCachePath()
	if err != nil {
		return
	}

	if latest := release.CachedLatest(cachePath); latest != "" {
		if release.IsOutdated(buildInfo.Version, latest) {
			printOutdatedWarning(cmd.ErrOrStderr(), buildInfo.Version, latest)
		}
		return
	}

	// Cache miss or expired. Refresh in the background — the first invocation
	// after an install gets no warning (we have nothing to compare to yet),
	// but subsequent runs do. Detached so we never block command exit; main()
	// waits up to ~200ms via WaitForBackgroundChecks before exiting so fast
	// commands still get a chance to populate the cache.
	refreshWG.Add(1)
	go func() {
		defer refreshWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()
		_, _ = release.RefreshCache(ctx, release.New(), cachePath)
	}()
}

func printOutdatedWarning(w io.Writer, current, latest string) {
	fmt.Fprintf(w, "warning: ackoctl %s is outdated (latest: %s). Run `ackoctl upgrade` to update.\n", current, latest)
}

// shouldSkipVersionCheck disables the check for commands that would either
// confuse it (`version`, `upgrade`) or are emitted from non-interactive
// contexts where a warning line corrupts machine-readable output.
func shouldSkipVersionCheck(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version", "upgrade", "help", "completion":
		return true
	}
	// Walk up to root looking for `completion` or `help` as ancestors —
	// `ackoctl completion bash` would otherwise sneak past the leaf check.
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" || c.Name() == "help" {
			return true
		}
	}
	return false
}
