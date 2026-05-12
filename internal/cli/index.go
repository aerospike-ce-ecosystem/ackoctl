package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newIndexCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage Aerospike secondary indexes",
	}
	cmd.AddCommand(
		newIndexListCmd(global),
		newIndexCreateCmd(global),
		newIndexDeleteCmd(global),
	)
	return cmd
}

func newIndexListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List secondary indexes across all namespaces",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			idx, err := c.ListIndexes(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, idx,
				output.WithTable(
					[]string{"NAMESPACE", "SET", "NAME", "BIN", "TYPE", "STATE"},
					func(v any) []string {
						i := v.(client.SecondaryIndex)
						return []string{i.Namespace, i.Set, i.Name, i.Bin, i.Type, i.State}
					},
					func(any) []any {
						rows := make([]any, 0, len(idx))
						for _, i := range idx {
							rows = append(rows, i)
						}
						return rows
					},
				),
			)
		},
	}
}

func newIndexCreateCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, bin, name, idxType string
	)
	cmd := &cobra.Command{
		Use:   "create CONN_ID",
		Short: "Create a secondary index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			idx, err := c.CreateIndex(cmd.Context(), args[0], client.CreateIndexRequest{
				Namespace: namespace,
				Set:       set,
				Bin:       bin,
				Name:      name,
				Type:      idxType,
			})
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, idx)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set (required)")
	cmd.Flags().StringVar(&bin, "bin", "", "bin name to index (required)")
	cmd.Flags().StringVar(&name, "name", "", "index name (required)")
	cmd.Flags().StringVar(&idxType, "type", "", "numeric|string|geo2dsphere (required)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("bin")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newIndexDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, name string
		yes             bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Delete a secondary index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteIndex(cmd.Context(), args[0], namespace, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted index %s in namespace %s\n", name, namespace)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&name, "name", "", "index name (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
