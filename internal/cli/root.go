package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// GlobalFlags holds values from persistent root flags, populated by cobra
// before any subcommand RunE fires. The *Explicit fields record whether the
// user supplied the flag on the command line, so override logic can
// distinguish "user said false" from "user said nothing".
type GlobalFlags struct {
	ConfigPath              string
	Context                 string
	Server                  string
	Token                   string
	WorkspaceID             string
	OutputFormat            string
	Verbose                 bool
	InsecureSkipTLS         bool
	NoVersionCheck          bool
	ContextExplicit         bool
	WorkspaceExplicit       bool
	WorkspaceEnvExplicit    bool
	InsecureSkipTLSExplicit bool
}

func (g *GlobalFlags) Format() (output.Format, error) {
	return output.Parse(g.OutputFormat)
}

func (g *GlobalFlags) WorkspaceSupplied() bool {
	return g.WorkspaceExplicit || g.WorkspaceEnvExplicit
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
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			f := c.Flags()
			flags.ContextExplicit = f.Changed("context")
			flags.WorkspaceExplicit = f.Changed("workspace")
			flags.WorkspaceEnvExplicit = os.Getenv(config.EnvWorkspace) != ""
			flags.InsecureSkipTLSExplicit = f.Changed("insecure-skip-tls")
			// Validate -o once, before any subcommand RunE builds a client or
			// makes a network call. Without this the only -o validation is the
			// per-command global.Format() call, which runs AFTER the API request
			// — so `record put ... -o xml` would mutate server state and then
			// exit 1, misleading the user into retrying a non-idempotent write.
			if _, err := output.Parse(flags.OutputFormat); err != nil {
				return err
			}
			runVersionCheck(c, flags.NoVersionCheck)
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "path to ackoctl config file (default $HOME/.ackoctl/config.yaml)")
	cmd.PersistentFlags().StringVar(&flags.Context, "context", "", "context name to use (overrides current-context)")
	cmd.PersistentFlags().StringVar(&flags.Server, "server", "", "cluster-manager API base URL (overrides context)")
	cmd.PersistentFlags().StringVar(&flags.Token, "token", "", "bearer token for cluster-manager (overrides context)")
	cmd.PersistentFlags().StringVar(&flags.WorkspaceID, "workspace", "", "cluster-manager workspace id for ACL scoping")
	cmd.PersistentFlags().StringVarP(&flags.OutputFormat, "output", "o", "table", "output format: table|json|yaml")
	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "verbose logging to stderr")
	cmd.PersistentFlags().BoolVar(&flags.InsecureSkipTLS, "insecure-skip-tls", false, "skip TLS certificate verification (dev only)")
	cmd.PersistentFlags().BoolVar(&flags.NoVersionCheck, "no-version-check", false, "disable the once-a-day check for a newer ackoctl release (also: ACKOCTL_NO_VERSION_CHECK=1)")

	cmd.AddCommand(
		newVersionCmd(),
		newUpgradeCmd(),
		newConfigCmd(flags),
		newConnectionCmd(flags),
		newClusterCmd(flags),
		newK8sCmd(flags),
		newRecordCmd(flags),
		newSetCmd(flags),
		newQueryCmd(flags),
		newIndexCmd(flags),
		newNoteCmd(flags),
		newGuideCmd(flags),
		newUdfCmd(flags),
		newAdminCmd(flags),
		newInfoCmd(flags),
	)
	return cmd
}
