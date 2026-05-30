package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// Issue #5 — `ackoctl set truncate` parity with cluster-manager's MCP
// truncate_set tool. Destructive: `--yes/-y` is mandatory.

// cluster-manager does not expose a dedicated /sets endpoint. We derive the
// set list from the cluster info response so the command stays robust
// against schema evolution; only the {namespaces:[{name, sets:[{name,...}]}]}
// shape is required.

type setRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Objects   any    `json:"objects,omitempty"`
	MemUsed   any    `json:"memUsed,omitempty"`
}

func newSetCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Inspect Aerospike sets (derived from cluster info)",
	}
	cmd.AddCommand(newSetListCmd(global), newSetTruncateCmd(global))
	return cmd
}

func newSetTruncateCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace string
		set       string
		beforeLut int64
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "truncate CONN_ID",
		Short: "Truncate every record in a set (optionally up to a last-update-time cutoff)",
		Long: `Truncate every record in <namespace>.<set>.

When --before-lut is omitted the entire set is wiped. When provided, only
records whose last-update-time is below the given nanosecond cutoff (since
the CITRUS epoch) are truncated. Server rejects --before-lut=0 outright to
avoid the silent "lut=0 means truncate-all" footgun at the info-command
layer; omit the flag for a full truncate.

Destructive: requires --yes/-y to proceed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			var lutPtr *int64
			if cmd.Flags().Changed("before-lut") {
				// Server rejects before_lut=0 as ambiguous and a negative
				// nanosecond cutoff is meaningless; fail fast client-side so the
				// error lands next to the typo rather than after a round-trip.
				if beforeLut <= 0 {
					return fmt.Errorf("--before-lut must be a positive nanosecond timestamp (omit for a full truncate); got %d", beforeLut)
				}
				v := beforeLut
				lutPtr = &v
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.TruncateSet(cmd.Context(), args[0], namespace, set, lutPtr); err != nil {
				return err
			}
			if lutPtr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Truncated %s/%s.%s up to lut=%d\n", args[0], namespace, set, *lutPtr)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Truncated %s/%s.%s (full set)\n", args[0], namespace, set)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name (required)")
	cmd.Flags().Int64Var(&beforeLut, "before-lut", 0, "truncate only records with last-update-time below this nanosecond cutoff; omit for a full set wipe")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive truncate")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	return cmd
}

func newSetListCmd(global *GlobalFlags) *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List sets across (or within) namespaces",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			info, err := c.ClusterInfo(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			var warnW io.Writer
			if global.Verbose {
				warnW = cmd.ErrOrStderr()
			}
			rows, drifted := extractSets(info, namespace, warnW)
			if drifted && len(rows) == 0 && hasNamespaceKey(info) {
				return fmt.Errorf("cluster info contained namespaces but none parsed — cluster-manager schema may have changed; rerun with -o json to inspect raw payload")
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, rows,
				output.WithTable(
					[]string{"NAMESPACE", "SET", "OBJECTS", "MEM_USED"},
					func(v any) []string {
						r := v.(setRow)
						return []string{r.Namespace, r.Name, cellString(r.Objects), cellString(r.MemUsed)}
					},
					func(any) []any {
						out := make([]any, 0, len(rows))
						for _, r := range rows {
							out = append(out, r)
						}
						return out
					},
				),
			)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "filter to a single namespace")
	return cmd
}

// extractSets walks the raw cluster-info map. Each silently-skipped element
// counts as schema drift; warnW receives a one-line warning per drift event
// when non-nil. The returned `drifted` flag lets callers escalate to an
// error if the cluster claimed namespaces but parsing produced zero rows.
func extractSets(info client.ClusterInfo, namespace string, warnW io.Writer) (rows []setRow, drifted bool) {
	rows = make([]setRow, 0)
	rawNs, ok := info["namespaces"]
	if !ok {
		return rows, false
	}
	nsList, ok := rawNs.([]any)
	if !ok {
		warn(warnW, "ackoctl: cluster info `namespaces` is not a list (got %T)", rawNs)
		return rows, true
	}
	for i, n := range nsList {
		ns, ok := n.(map[string]any)
		if !ok {
			warn(warnW, "ackoctl: namespaces[%d] is not an object (got %T)", i, n)
			drifted = true
			continue
		}
		nsName, _ := ns["name"].(string)
		if nsName == "" {
			warn(warnW, "ackoctl: namespaces[%d].name is missing or non-string", i)
			drifted = true
		}
		if namespace != "" && nsName != namespace {
			continue
		}
		rawSets, hasSets := ns["sets"]
		if !hasSets {
			continue
		}
		sets, ok := rawSets.([]any)
		if !ok {
			warn(warnW, "ackoctl: namespaces[%d].sets is not a list (got %T)", i, rawSets)
			drifted = true
			continue
		}
		for j, s := range sets {
			sm, ok := s.(map[string]any)
			if !ok {
				warn(warnW, "ackoctl: namespaces[%d].sets[%d] is not an object (got %T)", i, j, s)
				drifted = true
				continue
			}
			name, _ := sm["name"].(string)
			if name == "" {
				warn(warnW, "ackoctl: namespaces[%d].sets[%d].name is missing or non-string", i, j)
				drifted = true
			}
			rows = append(rows, setRow{
				Namespace: nsName,
				Name:      name,
				Objects:   coalesce(sm, "objects", "object_count"),
				MemUsed:   coalesce(sm, "memoryDataBytes", "memory_data_bytes", "memUsed", "memory_used", "data-used-bytes"),
			})
		}
	}
	return rows, drifted
}

func hasNamespaceKey(info client.ClusterInfo) bool {
	v, ok := info["namespaces"]
	if !ok {
		return false
	}
	if list, ok := v.([]any); ok {
		return len(list) > 0
	}
	return true
}

func warn(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}

func coalesce(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

// cellString renders a table cell for an `any` value. A nil value becomes an
// empty string so the table doesn't surface Go's `<nil>` to end users when a
// cluster-info field is absent or null.
func cellString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
