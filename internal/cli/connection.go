package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newConnectionCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection",
		Short: "Manage Aerospike connection profiles",
	}
	cmd.AddCommand(
		newConnectionListCmd(global),
		newConnectionGetCmd(global),
		newConnectionCreateCmd(global),
		newConnectionUpdateCmd(global),
		newConnectionDeleteCmd(global),
		newConnectionHealthCmd(global),
	)
	return cmd
}

func newConnectionListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connection profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			conns, err := c.ListConnections(cmd.Context(), c.Workspace)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, conns,
				output.WithTable(
					[]string{"ID", "NAME", "HOSTS", "PORT", "WORKSPACE", "NOTE"},
					func(v any) []string {
						c := v.(client.Connection)
						return []string{c.ID, c.Name, strings.Join(c.Hosts, ","), fmt.Sprint(c.Port), c.WorkspaceID, truncateNote(c.Note, noteDisplayLimit)}
					},
					func(any) []any {
						rows := make([]any, 0, len(conns))
						for _, c := range conns {
							rows = append(rows, c)
						}
						return rows
					},
				),
			)
		},
	}
}

func newConnectionGetCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get ID",
		Short: "Get a connection profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			conn, err := c.GetConnection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, conn)
		},
	}
}

func newConnectionCreateCmd(global *GlobalFlags) *cobra.Command {
	var (
		name        string
		hosts       []string
		port        int
		clusterName string
		username    string
		password    string
		color       string
		note        string
		labels      []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a connection profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cleanHosts, err := sanitizeHosts(hosts)
			if err != nil {
				return err
			}
			labelMap, err := parseLabels(labels)
			if err != nil {
				return err
			}
			req := client.CreateConnectionRequest{
				Name:        name,
				Hosts:       cleanHosts,
				Port:        port,
				ClusterName: clusterName,
				Username:    username,
				Password:    password,
				Color:       color,
				Note:        note,
				Labels:      labelMap,
				WorkspaceID: global.WorkspaceID,
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if req.WorkspaceID == "" && c.Workspace != "" && global.Verbose && !global.WorkspaceSupplied() {
				fmt.Fprintf(cmd.ErrOrStderr(), "ackoctl: creating connection in workspace=%s (from context; set --workspace to override)\n", c.Workspace)
			}
			conn, err := c.CreateConnection(cmd.Context(), req)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, conn)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "connection display name (required)")
	cmd.Flags().StringSliceVar(&hosts, "host", nil, "Aerospike node host (repeat for multi-host; required)")
	cmd.Flags().IntVar(&port, "port", 3000, "Aerospike service port")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "cluster name (TLS) — optional")
	cmd.Flags().StringVar(&username, "user", "", "auth username — optional")
	cmd.Flags().StringVar(&password, "pass", "", "auth password — optional")
	cmd.Flags().StringVar(&color, "color", "", "UI accent color in #RRGGBB — optional")
	cmd.Flags().StringVar(&note, "note", "", "free-form note")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "label as key=value (repeatable)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

func newConnectionUpdateCmd(global *GlobalFlags) *cobra.Command {
	var (
		name        string
		hosts       []string
		port        int
		clusterName string
		username    string
		password    string
		color       string
		note        string
		labels      []string
		workspaceID string
	)
	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a connection profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := client.UpdateConnectionRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("host") {
				cleanHosts, err := sanitizeHosts(hosts)
				if err != nil {
					return err
				}
				req.Hosts = cleanHosts
			}
			if cmd.Flags().Changed("port") {
				req.Port = &port
			}
			if cmd.Flags().Changed("cluster-name") {
				req.ClusterName = &clusterName
			}
			if cmd.Flags().Changed("user") {
				req.Username = &username
			}
			if cmd.Flags().Changed("pass") {
				req.Password = &password
			}
			if cmd.Flags().Changed("color") {
				req.Color = &color
			}
			if cmd.Flags().Changed("note") {
				req.Note = &note
			}
			if cmd.Flags().Changed("workspace-id") {
				req.WorkspaceID = &workspaceID
			}
			if cmd.Flags().Changed("label") {
				labelMap, err := parseLabels(labels)
				if err != nil {
					return err
				}
				req.Labels = labelMap
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			conn, err := c.UpdateConnection(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, conn)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new display name")
	cmd.Flags().StringSliceVar(&hosts, "host", nil, "replacement host list")
	cmd.Flags().IntVar(&port, "port", 0, "new port")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "new cluster name")
	cmd.Flags().StringVar(&username, "user", "", "new username")
	cmd.Flags().StringVar(&password, "pass", "", "new password")
	cmd.Flags().StringVar(&color, "color", "", "new accent color")
	cmd.Flags().StringVar(&note, "note", "", "new note")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "replace labels — key=value (repeatable)")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "move to a new workspace")
	return cmd
}

func newConnectionDeleteCmd(global *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a connection profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("refusing to delete %q without --yes", args[0])
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteConnection(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted connection %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	return cmd
}

func newConnectionHealthCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "health ID",
		Short: "Probe connection health",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			st, err := c.ConnectionHealth(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, st)
		},
	}
}

// sanitizeHosts trims whitespace and drops empty entries. Returns an error if
// the user supplied --host but nothing was left after cleanup, instead of
// sending [""] which cluster-manager rejects with a confusing validation
// error.
func sanitizeHosts(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h != "" {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--host must contain at least one non-empty value")
	}
	return out, nil
}

func parseLabels(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("invalid label %q (expected key=value)", p)
		}
		// strings.Cut("=value", "=") yields an empty key with ok=true; reject
		// it explicitly so a label like "=prod" cannot land an unnamed entry.
		if k == "" {
			return nil, fmt.Errorf("invalid label %q: key must not be empty", p)
		}
		out[k] = v
	}
	return out, nil
}
