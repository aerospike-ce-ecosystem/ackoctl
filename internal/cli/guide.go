package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// defaultWorkspaceID is cluster-manager's built-in workspace (DEFAULT_WORKSPACE_ID).
// Guides are workspace-scoped; when neither --workspace nor the current context
// supplies one, guide commands fall back to this well-known default rather than
// failing. Unlike "first workspace" (which the project forbids), ws-default
// always exists and is readable by every authenticated caller, so the fallback
// is deterministic — and it is announced on stderr under --verbose.
const defaultWorkspaceID = "ws-default"

// guideTypes are the guide kinds cluster-manager accepts. Kept in sync with
// the GuideType Literal in the backend models/guide.py.
var guideTypes = []string{"data-plane", "control-plane"}

func newGuideCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Read operational guides (org/team policy) from cluster-manager",
		Long: `Operational guides are workspace-scoped Markdown policy documents managed
in cluster-manager. Each workspace has a data-plane guide (policy for Aerospike
data CRUD) and a control-plane guide (policy for Aerospike cluster lifecycle).

Read the relevant guide BEFORE running data or cluster operations so your
changes follow the org/team policy:

  ackoctl guide get data-plane      # before record/set/query writes
  ackoctl guide get control-plane   # before creating/scaling/deleting clusters

Guides are authored by acko administrators in the cluster-manager web UI; this
command is read-only.`,
	}
	cmd.AddCommand(
		newGuideListCmd(global),
		newGuideGetCmd(global),
	)
	return cmd
}

// guideWorkspace resolves the workspace for guide commands, falling back to the
// built-in default when none is configured. The fallback is announced on
// stderr under --verbose so a user notices the scoping.
func guideWorkspace(cmd *cobra.Command, c *client.BaseClient) string {
	if c.Workspace != "" {
		return c.Workspace
	}
	// Announce the ws-default fallback unconditionally (on stderr, so it never
	// pollutes piped Markdown). With zero workspace configured the scope is a
	// genuine surprise — the user must be able to see which workspace was hit.
	fmt.Fprintf(cmd.ErrOrStderr(),
		"ackoctl: no workspace set, using %s (pass --workspace to override)\n",
		defaultWorkspaceID)
	return defaultWorkspaceID
}

func newGuideListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the operational guides registered for the workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			guides, err := c.ListGuides(cmd.Context(), guideWorkspace(cmd, c))
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, guides,
				output.WithTable(
					[]string{"TYPE", "TITLE", "UPDATED_AT", "UPDATED_BY"},
					func(v any) []string {
						g := v.(client.Guide)
						return []string{g.GuideType, g.Title, g.UpdatedAt, g.UpdatedBy}
					},
					func(any) []any {
						rows := make([]any, 0, len(guides))
						for _, g := range guides {
							rows = append(rows, g)
						}
						return rows
					},
				),
			)
		},
	}
}

func newGuideGetCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get GUIDE_TYPE",
		Short: "Read one operational guide (data-plane or control-plane)",
		Long: `Print an operational guide.

With the default (table) output the raw Markdown body is written to stdout, so
the guide reads naturally and pipes cleanly. -o json / -o yaml emit the full
structured guide (title, timestamps, author).

GUIDE_TYPE must be one of: data-plane, control-plane.`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: guideTypes,
		RunE: func(cmd *cobra.Command, args []string) error {
			guideType := args[0]
			if guideType != "data-plane" && guideType != "control-plane" {
				return fmt.Errorf(
					"invalid guide type %q: must be one of %s",
					guideType, strings.Join(guideTypes, ", "))
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			guide, err := c.GetGuide(cmd.Context(), guideWorkspace(cmd, c), guideType)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			// Default/table output prints the Markdown body verbatim so the
			// guide is human-readable and pipe-friendly. Structured formats
			// emit the full record (title, timestamps, author).
			if format == output.FormatTable {
				body := guide.Content
				if body == "" {
					return nil // empty guide body — print nothing, exit 0
				}
				if !strings.HasSuffix(body, "\n") {
					body += "\n"
				}
				_, err := io.WriteString(cmd.OutOrStdout(), body)
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, guide)
		},
	}
}
