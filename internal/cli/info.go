package cli

import (
	"fmt"
	"strings"

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
		yes        bool
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
forward any verb including write-capable ones such as set-config:.

--allow-write mutates a running cluster's configuration, so it additionally
requires --yes/-y. Read-only runs (the default) need no confirmation.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --allow-write selects cluster-manager's write passthrough, so a
			// set-config: can reach the live cluster. That is at least as
			// destructive as the eleven pre-existing sites that gate on --yes,
			// and the gate belongs on impact, not on the verb spelling.
			//
			// This was once the only guard on the path. It no longer is:
			// aerospike-cluster-manager#467 put a dependency on that route which
			// 403s unless the API runs with ACM_ALLOW_INFO_WRITE=true, and then
			// still admits only info_verbs.WRITE_INFO_VERBS — never
			// truncate-namespace. The client-side gate stays because the server
			// one is opt-in: on an API where an operator has turned it on, --yes
			// is again the last thing between a typo and a live cluster.
			//
			// --yes is the only affordance — there is no interactive prompt
			// anywhere in ackoctl, by design, so CI can never hang here.
			if allowWrite && !yes {
				return fmt.Errorf("confirmation required (--yes): --allow-write forwards write verbs to %s", mutationTarget(global))
			}
			// MarkFlagRequired only checks --command was supplied, not that it
			// carries a verb. Reject empty/whitespace values so the server is
			// never hit with a meaningless asinfo request.
			for _, verb := range commands {
				if strings.TrimSpace(verb) == "" {
					return fmt.Errorf("--command must not be empty")
				}
			}
			c, err := newConnClient(cmd, global, args[0])
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
							sanitizeCell(errStr),
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
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "bypass the read-only whitelist (allow set-config: and other write verbs); requires --yes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm live-cluster config mutation (required with --allow-write)")
	_ = cmd.MarkFlagRequired("command")
	return wsGuardedCmd(cmd)
}
