package cli

import (
	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// infoOutputLimit caps the per-row "OUTPUT" column width in the default
// table view. asinfo verbs like “statistics“ or “namespace/test“ return
// hundreds of semicolon-delimited stats that obliterate a terminal; the JSON
// and YAML formats preserve the full payload.
const infoOutputLimit = 80

func newInfoCmd(global *GlobalFlags) *cobra.Command {
	var (
		commands   []string
		node       string
		allowWrite bool
	)
	cmd := &cobra.Command{
		Use:   "info CONN_ID",
		Short: "Run asinfo commands against a cluster via cluster-manager passthrough",
		Long: `Execute one or more asinfo verbs against an Aerospike cluster. cluster-manager
runs the request on the cluster's client connection and returns one row per
(node, command) pair. Without --node the request fans out across every
reachable node.

By default the cluster-manager read-only whitelist is enforced (build,
status, statistics, namespaces, namespace/<ns>, ...); pass --allow-write to
forward any verb including write-capable ones such as set-config:.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			req := client.ExecuteInfoRequest{
				Commands: commands,
				Node:     node,
				// --allow-write inverts the default. Default is readOnly=true
				// so unsuspecting users cannot accidentally mutate config.
				ReadOnly: !allowWrite,
			}
			resp, err := c.ExecuteInfo(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			// JSON / YAML emit the full envelope; table flattens to rows.
			if format == output.FormatJSON || format == output.FormatYAML {
				return output.Print(cmd.OutOrStdout(), format, resp)
			}
			return output.Print(cmd.OutOrStdout(), format, resp.Results,
				output.WithTable(
					[]string{"NODE", "COMMAND", "OUTPUT", "ERROR"},
					func(v any) []string {
						r := v.(client.InfoCommandResult)
						errStr := ""
						if r.Error != nil {
							errStr = *r.Error
						}
						return []string{
							r.Node,
							r.Command,
							truncateNote(r.Output, infoOutputLimit),
							errStr,
						}
					},
					func(v any) []any {
						src := v.([]client.InfoCommandResult)
						rows := make([]any, 0, len(src))
						for _, r := range src {
							rows = append(rows, r)
						}
						return rows
					},
				),
			)
		},
	}
	// StringArrayVar (not StringSliceVar) so commas inside asinfo verbs are
	// preserved verbatim. ``--command "set-config:context=service;..."`` would
	// otherwise be split into pieces by StringSliceVar's comma-splitting.
	cmd.Flags().StringArrayVar(&commands, "command", nil, "asinfo verb to execute; repeatable (required)")
	cmd.Flags().StringVar(&node, "node", "", "target a single node by id (e.g. BB9020011AC4202); omit to fan out")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "bypass the read-only whitelist (allow set-config: and other write verbs)")
	_ = cmd.MarkFlagRequired("command")
	return cmd
}
