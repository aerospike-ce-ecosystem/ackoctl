package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newClusterCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Inspect and configure Aerospike clusters via cluster-manager",
	}
	cmd.AddCommand(
		newClusterInfoCmd(global),
		newClusterConfigureNamespaceCmd(global),
	)
	return cmd
}

func newClusterInfoCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "info CONN_ID",
		Short: "Show cluster nodes, namespaces, and sets",
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
			format, err := global.Format()
			if err != nil {
				return err
			}
			// ClusterInfo is a raw map — table view falls back to key:value dump.
			return output.Print(cmd.OutOrStdout(), format, info)
		},
	}
}

func newClusterConfigureNamespaceCmd(global *GlobalFlags) *cobra.Command {
	var (
		nsName string
		params []string
	)
	cmd := &cobra.Command{
		Use:   "configure-namespace CONN_ID",
		Short: "Tune runtime-mutable params of an existing Aerospike namespace",
		Long: `cluster-manager applies dynamic config changes via asinfo set-config.
Namespaces cannot be created at runtime — they must be defined in aerospike.conf.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := client.ConfigureNamespaceRequest{"namespace": nsName}
			for _, p := range params {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return fmt.Errorf("invalid --param %q (expected key=value)", p)
				}
				req[k] = v
			}
			c, err := newClient(global)
			if err != nil {
				return err
			}
			msg, err := c.ConfigureNamespace(context.Background(), args[0], req)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&nsName, "name", "", "namespace name (required)")
	cmd.Flags().StringSliceVar(&params, "param", nil, "runtime-tunable parameter as key=value (repeatable)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
