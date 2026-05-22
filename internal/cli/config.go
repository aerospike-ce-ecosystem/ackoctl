package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newConfigCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ackoctl configuration contexts",
	}
	cmd.AddCommand(
		newConfigViewCmd(global),
		newConfigSetContextCmd(global),
		newConfigUseContextCmd(global),
		newConfigCurrentContextCmd(global),
		newConfigDeleteContextCmd(global),
	)
	return cmd
}

func resolveConfigPath(global *GlobalFlags) (string, error) {
	if global.ConfigPath != "" {
		return global.ConfigPath, nil
	}
	return config.DefaultPath()
}

// redactedToken is the placeholder substituted for a non-empty bearer token
// in `config view` output so credentials never reach the terminal or a piped
// json/yaml consumer.
const redactedToken = "***"

// redactConfigTokens returns a shallow copy of cfg with every non-empty
// context Token replaced by redactedToken. The input config is not mutated,
// so the on-disk file written by config.Save still carries the real token.
func redactConfigTokens(cfg *config.Config) *config.Config {
	out := *cfg
	out.Contexts = make([]config.Context, len(cfg.Contexts))
	for i, c := range cfg.Contexts {
		if c.Token != "" {
			c.Token = redactedToken
		}
		out.Contexts[i] = c
	}
	return &out
}

func newConfigViewCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show merged ackoctl config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveConfigPath(global)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			// Redact bearer tokens before marshaling. The table view never
			// prints the token, but -o json / -o yaml would otherwise expose
			// it in plaintext. This only affects displayed output — the
			// on-disk config file written by config.Save is untouched.
			cfg = redactConfigTokens(cfg)
			return output.Print(cmd.OutOrStdout(), format, cfg,
				output.WithTable(
					[]string{"CURRENT", "NAME", "SERVER", "WORKSPACE"},
					func(v any) []string {
						c := v.(config.Context)
						marker := ""
						if cfg.CurrentContext == c.Name {
							marker = "*"
						}
						return []string{marker, c.Name, c.Server, c.WorkspaceID}
					},
					func(any) []any {
						out := make([]any, 0, len(cfg.Contexts))
						for _, c := range cfg.Contexts {
							out = append(out, c)
						}
						return out
					},
				),
			)
		},
	}
}

func newConfigSetContextCmd(global *GlobalFlags) *cobra.Command {
	var (
		server      string
		token       string
		workspaceID string
		insecure    bool
	)
	cmd := &cobra.Command{
		Use:   "set-context NAME",
		Short: "Create or update a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path, err := resolveConfigPath(global)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			existing, _ := cfg.Find(name)
			merged := config.Context{Name: name}
			if existing != nil {
				merged = *existing
				merged.Name = name
			}
			if cmd.Flags().Changed("server") {
				merged.Server = server
			}
			if cmd.Flags().Changed("token") {
				merged.Token = token
			}
			if cmd.Flags().Changed("workspace-id") {
				merged.WorkspaceID = workspaceID
			}
			if cmd.Flags().Changed("insecure-skip-tls") {
				merged.InsecureSkipTLS = insecure
			}
			if merged.Server == "" {
				return fmt.Errorf("--server is required when creating a new context")
			}
			cfg.Upsert(merged)
			if cfg.CurrentContext == "" {
				cfg.CurrentContext = name
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Context %q saved to %s\n", name, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "cluster-manager API base URL (e.g. http://localhost:8000/api)")
	cmd.Flags().StringVar(&token, "token", "", "bearer token (obtain via your IdP)")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "default workspace id for this context")
	cmd.Flags().BoolVar(&insecure, "insecure-skip-tls", false, "skip TLS certificate verification for this context")
	return cmd
}

func newConfigUseContextCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use-context NAME",
		Short: "Set the current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(global)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := cfg.Use(args[0]); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q\n", args[0])
			return nil
		},
	}
}

func newConfigCurrentContextCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Print the current context name",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveConfigPath(global)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if cfg.CurrentContext == "" {
				return config.ErrNoCurrent
			}
			fmt.Fprintln(cmd.OutOrStdout(), cfg.CurrentContext)
			return nil
		},
	}
}

func newConfigDeleteContextCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-context NAME",
		Short: "Remove a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(global)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := cfg.Delete(args[0]); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted context %q\n", args[0])
			return nil
		},
	}
}
