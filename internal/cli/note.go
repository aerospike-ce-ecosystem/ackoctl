package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// noteDisplayLimit is the column width used when rendering note bodies in
// table output. Notes can be up to 8 KB (MAX_NOTE_LENGTH on the server);
// printing every byte makes the table unreadable, so we truncate with an
// ellipsis. JSON/YAML output preserves the full body.
const noteDisplayLimit = 60

func newNoteCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage operator notes on Aerospike sets and records",
		Long: `Notes are free-text memos stored in cluster-manager's metaDB (not in
Aerospike itself). They are scoped per connection profile and cascade-delete
with the connection. Use them to attach runbook context, ticket references,
or known-issue annotations to sets or specific records.`,
	}
	cmd.AddCommand(
		newNoteSetCmd(global),
		newNoteRecordCmd(global),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// set notes
// ---------------------------------------------------------------------------

func newNoteSetCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Manage set-level notes",
	}
	cmd.AddCommand(
		newNoteSetUpdateCmd(global),
		newNoteSetDeleteCmd(global),
		newNoteSetListCmd(global),
	)
	return cmd
}

func newNoteSetUpdateCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, note string
	)
	cmd := &cobra.Command{
		Use:   "update CONN_ID",
		Short: "Create or update the operator note attached to a set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.UpsertSetNote(cmd.Context(), args[0], namespace, set, note)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, out)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	cmd.Flags().StringVar(&note, "note", "", "note body — non-empty, up to 8 KB (required)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("note")
	return cmd
}

func newNoteSetDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set string
		yes            bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Remove the operator note attached to a set (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteSetNote(cmd.Context(), args[0], namespace, set); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted set note %s/%s.%s\n", args[0], namespace, set)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	return cmd
}

func newNoteSetListCmd(global *GlobalFlags) *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List set-level notes for a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			notes, err := c.ListSetNotes(cmd.Context(), args[0], namespace)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, notes,
				output.WithTable(
					[]string{"NAMESPACE", "SET", "NOTE", "UPDATED_AT", "UPDATED_BY"},
					func(v any) []string {
						n := v.(client.SetNote)
						return []string{n.Namespace, n.SetName, truncateNote(n.Note, noteDisplayLimit), n.UpdatedAt, n.UpdatedBy}
					},
					func(any) []any {
						rows := make([]any, 0, len(notes))
						for _, n := range notes {
							rows = append(rows, n)
						}
						return rows
					},
				),
			)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "filter to a single namespace (optional)")
	return cmd
}

// ---------------------------------------------------------------------------
// record notes
// ---------------------------------------------------------------------------

func newNoteRecordCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Manage record-level notes",
	}
	cmd.AddCommand(
		newNoteRecordUpdateCmd(global),
		newNoteRecordDeleteCmd(global),
		newNoteRecordListCmd(global),
	)
	return cmd
}

func newNoteRecordUpdateCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, pkType, note string
	)
	cmd := &cobra.Command{
		Use:   "update CONN_ID",
		Short: "Create or update the operator note on a single record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePKType(pkType); err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.UpsertRecordNote(cmd.Context(), args[0], namespace, set, pk, pkType, note)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, out)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	cmd.Flags().StringVar(&note, "note", "", "note body — non-empty, up to 8 KB (required)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	_ = cmd.MarkFlagRequired("note")
	return cmd
}

func newNoteRecordDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, pkType string
		yes                        bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Remove the operator note on a single record (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			if err := validatePKType(pkType); err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteRecordNote(cmd.Context(), args[0], namespace, set, pk, pkType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted record note %s/%s.%s pk=%s\n", args[0], namespace, set, pk)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	return cmd
}

func newNoteRecordListCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set string
	)
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List record-level notes for a (connection, namespace, set)",
		Long: `This is the recovery path for record notes that the random-50 data browser
scan does not surface — it returns every annotated record key for the slice.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			notes, err := c.ListRecordNotes(cmd.Context(), args[0], namespace, set)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, notes,
				output.WithTable(
					[]string{"NAMESPACE", "SET", "PK", "PK_TYPE", "NOTE", "UPDATED_AT"},
					func(v any) []string {
						n := v.(client.RecordNote)
						return []string{n.Namespace, n.SetName, n.PKText, n.PKType, truncateNote(n.Note, noteDisplayLimit), n.UpdatedAt}
					},
					func(any) []any {
						rows := make([]any, 0, len(notes))
						for _, n := range notes {
							rows = append(rows, n)
						}
						return rows
					},
				),
			)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	return cmd
}

// truncateNote shortens a note body to “limit“ runes (not bytes) and
// appends an ellipsis when truncation occurs. Operating on runes keeps
// multibyte UTF-8 (e.g. Korean) from getting sliced mid-codepoint.
func truncateNote(s string, limit int) string {
	if limit <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
