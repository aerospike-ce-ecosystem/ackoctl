package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

// resolveContext builds the effective Context applying file < env < flag.
// --insecure-skip-tls is honored as a real override only when the user
// supplied it on the CLI (Changed=true); otherwise CLI bool defaults would
// silently force the context value to false.
func resolveContext(global *GlobalFlags) (config.Context, error) {
	path, err := resolveConfigPath(global)
	if err != nil {
		return config.Context{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Context{}, err
	}
	env, err := config.EnvOverrides()
	if err != nil {
		return config.Context{}, err
	}
	flags := config.Overrides{
		Context:         global.Context,
		Server:          global.Server,
		Token:           global.Token,
		WorkspaceID:     global.WorkspaceID,
		ContextExplicit: global.ContextExplicit,
	}
	if global.InsecureSkipTLSExplicit {
		v := global.InsecureSkipTLS
		flags.InsecureSkipTLS = &v
	}
	return config.Resolve(cfg, env, flags)
}

// newClient builds a BaseClient from the merged Context and wires verbose
// logging into stderr when --verbose is on. cmd is used to discover the
// session's stderr writer (cobra plumbs OutOrStderr for testability).
func newClient(cmd *cobra.Command, global *GlobalFlags) (*client.BaseClient, error) {
	ctx, err := resolveContext(global)
	if err != nil {
		return nil, err
	}
	c := client.New(ctx)
	if global.Verbose {
		c.VerboseLogger = cmd.ErrOrStderr()
		if c.HTTPClient != nil && ctx.InsecureSkipTLS {
			fmt.Fprintln(c.VerboseLogger, "ackoctl: WARNING — TLS verification is disabled")
		}
	}
	// Surface workspace fallback so users notice when an ACL is silently
	// scoping their request to the current context's workspace.
	if !global.WorkspaceSupplied() && ctx.WorkspaceID != "" {
		warnWorkspaceFallback(cmd.ErrOrStderr(), global.Verbose, ctx.WorkspaceID)
	}
	return c, nil
}

func warnWorkspaceFallback(w io.Writer, verbose bool, ws string) {
	if !verbose {
		return
	}
	fmt.Fprintf(w, "ackoctl: using workspace=%s from current context (set --workspace to override)\n", ws)
}
