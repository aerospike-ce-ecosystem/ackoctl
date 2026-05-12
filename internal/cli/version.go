package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var buildInfo = BuildInfo{Version: "dev", Commit: "none", Date: "unknown"}

func SetBuildInfo(b BuildInfo) {
	buildInfo = b
}

func newVersionCmd() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print ackoctl version",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if short {
				fmt.Fprintln(out, buildInfo.Version)
				return nil
			}
			fmt.Fprintf(out, "ackoctl version %s\n", buildInfo.Version)
			fmt.Fprintf(out, "  commit:  %s\n", buildInfo.Commit)
			fmt.Fprintf(out, "  built:   %s\n", buildInfo.Date)
			fmt.Fprintf(out, "  go:      %s\n", runtime.Version())
			fmt.Fprintf(out, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "print only the version string")
	return cmd
}
