package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// missingFieldSentinel marks a table cell where the server response did not
// include the expected key. Distinguishing missing from empty helps users
// spot schema drift instead of assuming the resource has no value.
const missingFieldSentinel = "<missing>"

func newK8sCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Operate on ACKO-managed Kubernetes Aerospike clusters",
		Long: `Requires cluster-manager to have K8S_MANAGEMENT_ENABLED=true.
When disabled, every k8s subcommand returns HTTP 404 from the server.`,
	}
	cmd.AddCommand(newK8sClusterCmd(global))
	return cmd
}

func newK8sClusterCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage AerospikeCluster CRs",
	}
	cmd.AddCommand(
		newK8sClusterListCmd(global),
		newK8sClusterGetCmd(global),
		newK8sClusterReconcileCmd(global),
	)
	return cmd
}

func newK8sClusterListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ACKO-managed clusters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			items, err := c.ListK8sClusters(cmd.Context())
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, items,
				output.WithTable(
					[]string{"NAMESPACE", "NAME", "PHASE", "NODES"},
					func(v any) []string {
						row := v.(client.K8sCluster)
						return []string{
							stringField(row, "namespace"),
							stringField(row, "name"),
							stringField(row, "phase"),
							stringField(row, "size"),
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(items))
						for _, it := range items {
							rows = append(rows, it)
						}
						return rows
					},
				),
			)
		},
	}
}

func newK8sClusterGetCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get NAMESPACE/NAME",
		Short: "Get a single ACKO-managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			cluster, err := c.GetK8sCluster(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, cluster)
		},
	}
}

func newK8sClusterReconcileCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile NAMESPACE/NAME",
		Short: "Force the ACKO operator to re-reconcile this cluster",
		Long: `Adds the acko.io/force-reconcile annotation to the AerospikeCluster CR.
Useful when the cluster is stuck in a drifted state.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.ForceReconcileK8sCluster(cmd.Context(), ns, name)
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
}

func splitNamespacedName(s string) (string, string, error) {
	ns, name, ok := strings.Cut(s, "/")
	if !ok || ns == "" || name == "" {
		return "", "", fmt.Errorf("expected NAMESPACE/NAME, got %q", s)
	}
	return ns, name, nil
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return missingFieldSentinel
	}
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
