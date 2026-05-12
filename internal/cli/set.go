package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

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
	cmd.AddCommand(newSetListCmd(global))
	return cmd
}

func newSetListCmd(global *GlobalFlags) *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List sets across (or within) namespaces",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(global)
			if err != nil {
				return err
			}
			info, err := c.ClusterInfo(context.Background(), args[0])
			if err != nil {
				return err
			}
			rows := extractSets(info, namespace)
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

func extractSets(info client.ClusterInfo, namespace string) []setRow {
	nsList, _ := info["namespaces"].([]any)
	rows := make([]setRow, 0)
	for _, n := range nsList {
		ns, _ := n.(map[string]any)
		if ns == nil {
			continue
		}
		nsName, _ := ns["name"].(string)
		if namespace != "" && nsName != namespace {
			continue
		}
		sets, _ := ns["sets"].([]any)
		for _, s := range sets {
			sm, _ := s.(map[string]any)
			if sm == nil {
				continue
			}
			name, _ := sm["name"].(string)
			rows = append(rows, setRow{
				Namespace: nsName,
				Name:      name,
				Objects:   coalesce(sm, "objects", "object_count"),
				MemUsed:   coalesce(sm, "memoryDataBytes", "memory_data_bytes", "memUsed", "memory_used", "data-used-bytes"),
			})
		}
	}
	return rows
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
