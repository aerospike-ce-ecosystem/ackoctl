package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// newAdminCmd groups Aerospike security-mode administration: user and role
// management against the cluster-manager “/admin/{conn_id}/*“ surface.
// CE has no security module, so calls against a CE target return a 5xx with
// "security not enabled" — the CLI surfaces that verbatim. The command ships
// because the same workspace can target Enterprise clusters.
func newAdminCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage Aerospike security users and roles",
		Long: `Administer Aerospike's built-in security users and roles via
cluster-manager. Requires the target cluster to have security enabled in
aerospike.conf — Community Edition does not ship the security module and
will reject every admin call.`,
	}
	cmd.AddCommand(
		newAdminUserCmd(global),
		newAdminRoleCmd(global),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// admin user
// ---------------------------------------------------------------------------

func newAdminUserCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage Aerospike users",
	}
	cmd.AddCommand(
		newAdminUserListCmd(global),
		newAdminUserCreateCmd(global),
		newAdminUserPasswdCmd(global),
		newAdminUserDeleteCmd(global),
	)
	return cmd
}

func newAdminUserListCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List Aerospike users",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			users, err := c.ListAdminUsers(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, users,
				output.WithTable(
					[]string{"USERNAME", "ROLES", "READ_QUOTA", "WRITE_QUOTA", "CONNECTIONS"},
					func(v any) []string {
						u := v.(client.AerospikeUser)
						return []string{
							u.Username,
							strings.Join(u.Roles, ","),
							formatOptInt(u.ReadQuota),
							formatOptInt(u.WriteQuota),
							formatOptInt(u.Connections),
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(users))
						for _, u := range users {
							rows = append(rows, u)
						}
						return rows
					},
				),
			)
		},
	}
	return cmd
}

func newAdminUserCreateCmd(global *GlobalFlags) *cobra.Command {
	var (
		username, password string
		passwordStdin      bool
		roles              []string
	)
	cmd := &cobra.Command{
		Use:   "create CONN_ID",
		Short: "Create a new Aerospike user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pw, err := resolvePassword(cmd.InOrStdin(), password, passwordStdin)
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.CreateAdminUser(cmd.Context(), args[0], client.CreateUserRequest{
				Username: username,
				Password: pw,
				Roles:    roles,
			})
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
	cmd.Flags().StringVar(&username, "username", "", "username (required)")
	cmd.Flags().StringVar(&password, "password", "", "password in plaintext — visible in shell history; prefer --password-stdin")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (mutually exclusive with --password)")
	cmd.Flags().StringSliceVar(&roles, "roles", nil, "comma-separated role names to assign (optional)")
	_ = cmd.MarkFlagRequired("username")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")
	cmd.MarkFlagsOneRequired("password", "password-stdin")
	return cmd
}

func newAdminUserPasswdCmd(global *GlobalFlags) *cobra.Command {
	var (
		username, password string
		passwordStdin      bool
	)
	cmd := &cobra.Command{
		Use:   "passwd CONN_ID",
		Short: "Change an existing user's password",
		Long: `Update only the password for an Aerospike user. Roles, quotas, and
whitelists are untouched - the server's PATCH endpoint is intentionally
password-only.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pw, err := resolvePassword(cmd.InOrStdin(), password, passwordStdin)
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			msg, err := c.ChangeAdminUserPassword(cmd.Context(), args[0], client.ChangePasswordRequest{
				Username: username,
				Password: pw,
			})
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, msg)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username (required)")
	cmd.Flags().StringVar(&password, "password", "", "new password in plaintext — visible in shell history; prefer --password-stdin")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (mutually exclusive with --password)")
	_ = cmd.MarkFlagRequired("username")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")
	cmd.MarkFlagsOneRequired("password", "password-stdin")
	return cmd
}

func newAdminUserDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		username string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Delete an Aerospike user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteAdminUser(cmd.Context(), args[0], username); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted user %s on %s\n", username, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

// ---------------------------------------------------------------------------
// admin role
// ---------------------------------------------------------------------------

func newAdminRoleCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Manage Aerospike roles",
	}
	cmd.AddCommand(
		newAdminRoleListCmd(global),
		newAdminRoleCreateCmd(global),
		newAdminRoleDeleteCmd(global),
	)
	return cmd
}

func newAdminRoleListCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List Aerospike roles",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			roles, err := c.ListAdminRoles(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, roles,
				output.WithTable(
					[]string{"NAME", "PRIVILEGES", "WHITELIST", "READ_QUOTA", "WRITE_QUOTA"},
					func(v any) []string {
						r := v.(client.AerospikeRole)
						return []string{
							r.Name,
							formatPrivileges(r.Privileges),
							strings.Join(r.Whitelist, ","),
							formatOptInt(r.ReadQuota),
							formatOptInt(r.WriteQuota),
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(roles))
						for _, r := range roles {
							rows = append(rows, r)
						}
						return rows
					},
				),
			)
		},
	}
	return cmd
}

func newAdminRoleCreateCmd(global *GlobalFlags) *cobra.Command {
	var (
		name           string
		privilegeSpecs []string
		whitelist      []string
		readQuota      int
		writeQuota     int
	)
	cmd := &cobra.Command{
		Use:   "create CONN_ID",
		Short: "Create a new Aerospike role",
		Long: `Create an Aerospike role with one or more privileges. Each --privilege
takes the form CODE (cluster-wide), CODE:NAMESPACE (namespace-scoped),
or CODE:NAMESPACE/SET (set-scoped). Repeat --privilege to attach multiple
privileges to one role.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			privs, err := parsePrivileges(privilegeSpecs)
			if err != nil {
				return err
			}
			req := client.CreateRoleRequest{
				Name:       name,
				Privileges: privs,
				Whitelist:  whitelist,
			}
			if cmd.Flags().Changed("read-quota") {
				v := readQuota
				req.ReadQuota = &v
			}
			if cmd.Flags().Changed("write-quota") {
				v := writeQuota
				req.WriteQuota = &v
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.CreateAdminRole(cmd.Context(), args[0], req)
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
	cmd.Flags().StringVar(&name, "name", "", "role name (required)")
	cmd.Flags().StringArrayVar(&privilegeSpecs, "privilege", nil, "privilege spec CODE[:NAMESPACE[/SET]] (repeatable, required)")
	cmd.Flags().StringSliceVar(&whitelist, "whitelist", nil, "comma-separated CIDR whitelist (optional)")
	cmd.Flags().IntVar(&readQuota, "read-quota", 0, "read TPS quota (optional)")
	cmd.Flags().IntVar(&writeQuota, "write-quota", 0, "write TPS quota (optional)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("privilege")
	return cmd
}

func newAdminRoleDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		name string
		yes  bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Delete an Aerospike role",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteAdminRole(cmd.Context(), args[0], name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted role %s on %s\n", name, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "role name (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// parsePrivileges turns CLI “--privilege“ strings into the wire shape.
// Accepted forms:
//   - “CODE“                  → {code: CODE}
//   - “CODE:NAMESPACE“        → {code: CODE, namespace: NAMESPACE}
//   - “CODE:NAMESPACE/SET“    → {code: CODE, namespace: NAMESPACE, set: SET}
//
// Empty CODE is rejected; whitespace is trimmed from each segment. Server-side
// validation is the authority on whether a CODE is a real privilege —
// ackoctl only checks structural shape.
func parsePrivileges(specs []string) ([]client.RolePrivilege, error) {
	out := make([]client.RolePrivilege, 0, len(specs))
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			return nil, fmt.Errorf("--privilege value must not be empty")
		}
		code, rest, hasNS := strings.Cut(spec, ":")
		code = strings.TrimSpace(code)
		if code == "" {
			return nil, fmt.Errorf("--privilege %q: code is required", raw)
		}
		p := client.RolePrivilege{Code: code}
		if hasNS {
			ns, set, hasSet := strings.Cut(rest, "/")
			ns = strings.TrimSpace(ns)
			if ns == "" {
				return nil, fmt.Errorf("--privilege %q: namespace is empty after ':'", raw)
			}
			// Reject embedded delimiters: a second ':' in the namespace
			// section indicates a malformed spec like "read::" or "read:a:b"
			// that previous code silently accepted (ns became ":" or "a:b").
			if strings.ContainsAny(ns, ":/") {
				return nil, fmt.Errorf("--privilege %q: namespace must not contain ':' or '/'", raw)
			}
			p.Namespace = ns
			if hasSet {
				set = strings.TrimSpace(set)
				if set == "" {
					return nil, fmt.Errorf("--privilege %q: set is empty after '/'", raw)
				}
				// Reject an extra '/' in the set section, mirroring the
				// namespace guard above. strings.Cut(rest, "/") folds any
				// trailing segment into set, so "read:ns/set/extra" silently
				// yielded set = "set/extra" — a set name Aerospike can never
				// have. That bogus set then round-trips to cluster-manager and
				// surfaces as a confusing server error far from the typo.
				if strings.Contains(set, "/") {
					return nil, fmt.Errorf("--privilege %q: set must not contain '/'", raw)
				}
				p.Set = set
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// formatPrivileges renders the wire shape back to the CLI input format for
// table output, so users can copy a role's privilege list verbatim into a
// subsequent “--privilege“ flag.
func formatPrivileges(privs []client.RolePrivilege) string {
	parts := make([]string, 0, len(privs))
	for _, p := range privs {
		switch {
		case p.Namespace != "" && p.Set != "":
			parts = append(parts, p.Code+":"+p.Namespace+"/"+p.Set)
		case p.Namespace != "":
			parts = append(parts, p.Code+":"+p.Namespace)
		default:
			parts = append(parts, p.Code)
		}
	}
	return strings.Join(parts, ",")
}

// resolvePassword merges --password and --password-stdin. Exactly one of the
// two must be supplied; combining both is a user error. The stdin path reads
// a single line (trailing newline stripped) so “echo "pw" | ackoctl ...“
// works as kubectl users expect.
func resolvePassword(stdin io.Reader, password string, passwordStdin bool) (string, error) {
	if password != "" && passwordStdin {
		return "", fmt.Errorf("--password and --password-stdin are mutually exclusive")
	}
	if passwordStdin {
		if stdin == nil {
			return "", fmt.Errorf("--password-stdin set but no stdin reader available")
		}
		reader := bufio.NewReader(stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read --password-stdin: %w", err)
		}
		pw := strings.TrimRight(line, "\r\n")
		if pw == "" {
			return "", fmt.Errorf("--password-stdin received empty password")
		}
		return pw, nil
	}
	if password == "" {
		return "", fmt.Errorf("--password or --password-stdin is required")
	}
	return password, nil
}

// formatOptInt renders an *int as a string for table output. nil becomes
// "-" so columns line up; otherwise we render the integer.
func formatOptInt(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}
