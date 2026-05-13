package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/release"
)

// upgradeTimeout caps the full self-upgrade flow. Generous because the
// archive download dominates and varies with link quality, but bounded so a
// flaky network never leaves the user staring at a frozen terminal.
const upgradeTimeout = 2 * time.Minute

func newUpgradeCmd() *cobra.Command {
	var (
		targetVersion string
		check         bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade ackoctl in place to the latest release",
		Long: `Replace the currently running ackoctl binary with the latest GitHub
release (or a specific version when --version is given).

The new binary is downloaded from the GitHub Releases page, its sha256 is
verified against checksums.txt, and the file is atomically renamed into
place. The running process keeps executing the old inode until it exits,
so it is safe to run while ackoctl is in use elsewhere.

If the destination is not writable (typically /usr/local/bin/ackoctl), the
command fails with a clear message — re-run with sudo or move the binary
to a path you own (e.g. ~/.local/bin).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), upgradeTimeout)
			defer cancel()

			client := release.New()
			currentVersion := buildInfo.Version

			tag := targetVersion
			if tag == "" {
				resolved, err := client.LatestTag(ctx)
				if err != nil {
					return err
				}
				tag = resolved
			} else if tag[0] != 'v' {
				tag = "v" + tag
			}

			out := cmd.OutOrStdout()

			if check {
				fmt.Fprintf(out, "current: %s\nlatest:  %s\n", currentVersion, tag)
				switch {
				case release.IsDevBuild(currentVersion):
					fmt.Fprintln(out, "running a dev build — version comparison skipped")
				case release.IsOutdated(currentVersion, tag):
					fmt.Fprintln(out, "an upgrade is available — run `ackoctl upgrade` to install")
				default:
					fmt.Fprintln(out, "already up to date")
				}
				return nil
			}

			if currentVersion == tag && !release.IsDevBuild(currentVersion) {
				fmt.Fprintf(out, "ackoctl %s is already the latest release\n", currentVersion)
				return nil
			}

			dst, err := release.CurrentExecutable()
			if err != nil {
				return err
			}

			tmpDir, err := os.MkdirTemp("", "ackoctl-upgrade-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			fmt.Fprintf(out, "downloading ackoctl %s for %s/%s …\n", tag, release.GOOS, release.GOARCH)
			binPath, err := client.DownloadAndExtract(ctx, tag, release.GOOS, release.GOARCH, tmpDir)
			if err != nil {
				return err
			}

			if err := release.Replace(binPath, dst); err != nil {
				if errors.Is(err, fs.ErrPermission) {
					return fmt.Errorf("write %s: permission denied — re-run with sudo or set $PATH to a directory you own", dst)
				}
				return err
			}
			fmt.Fprintf(out, "upgraded %s -> %s (installed to %s)\n", currentVersion, tag, dst)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetVersion, "version", "", "pin a specific release (e.g. v0.1.0); defaults to the latest tag")
	cmd.Flags().BoolVar(&check, "check", false, "only report current vs latest; do not download or replace")
	return cmd
}
