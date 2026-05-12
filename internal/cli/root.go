package cli

import (
	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// GlobalFlags holds values from persistent root flags, populated by cobra
// before any subcommand RunE fires.
type GlobalFlags struct {
	ConfigPath      string
	Context         string
	Server          string
	Token           string
	WorkspaceID     string
	OutputFormat    string
	Verbose         bool
	InsecureSkipTLS bool
}

func (g *GlobalFlags) Format() (output.Format, error) {
	return output.Parse(g.OutputFormat)
}

func NewRootCmd() *cobra.Command {
	flags := &GlobalFlags{}

	cmd := &cobra.Command{
		Use:   "ackoctl",
		Short: "Command-line interface for aerospike-cluster-manager",
		Long: `ackoctl talks to the aerospike-cluster-manager REST API to manage
Aerospike connections, browse records, run queries, and trigger ACKO
reconciliations.

It does not talk to Kubernetes or Aerospike directly — all operations
go through cluster-manager's /api/* surface.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "path to ackoctl config file (default $HOME/.ackoctl/config.yaml)")
	cmd.PersistentFlags().StringVar(&flags.Context, "context", "", "context name to use (overrides current-context)")
	cmd.PersistentFlags().StringVar(&flags.Server, "server", "", "cluster-manager API base URL (overrides context)")
	cmd.PersistentFlags().StringVar(&flags.Token, "token", "", "bearer token for cluster-manager (overrides context)")
	cmd.PersistentFlags().StringVar(&flags.WorkspaceID, "workspace", "", "cluster-manager workspace id for ACL scoping")
	cmd.PersistentFlags().StringVarP(&flags.OutputFormat, "output", "o", "table", "output format: table|json|yaml")
	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "verbose logging to stderr")
	cmd.PersistentFlags().BoolVar(&flags.InsecureSkipTLS, "insecure-skip-tls", false, "skip TLS certificate verification (dev only)")

	cmd.AddCommand(
		newVersionCmd(),
		newConfigCmd(flags),
	)
	return cmd
}
