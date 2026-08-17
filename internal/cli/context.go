package cli

import (
	"fmt"
	"io"
	"net/url"

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
	}
	// Unconditional, not gated on --verbose: `config set-context
	// --insecure-skip-tls` persists the setting, so an operator who enabled it
	// once for kind keeps sending the bearer token over an unverified
	// connection after the context is repointed at a real server. The warning
	// only earns its keep if it fires on every such run. It goes to stderr so
	// `-o json | jq` pipelines stay parseable.
	if ctx.InsecureSkipTLS {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"ackoctl: WARNING — TLS verification is disabled for %s; the bearer token is sent over an unverified connection\n",
			describeTarget(ctx))
	}
	// Surface workspace fallback so users notice when an ACL is silently
	// scoping their request to the current context's workspace.
	if !global.WorkspaceSupplied() && ctx.WorkspaceID != "" {
		warnWorkspaceFallback(cmd.ErrOrStderr(), global.Verbose, ctx.WorkspaceID)
	}
	return c, nil
}

// describeTarget renders a resolved Context as "context "prod" (acm.example.com)"
// for warnings and confirmation refusals, so the operator can tell which
// environment a command was about to touch. The Name is empty when the server
// came only from --server / $ACKOCTL_SERVER with no context on disk, so the
// host alone is reported in that case. Only the host is shown — the path and
// query would add noise, and the token is never part of the URL.
func describeTarget(ctx config.Context) string {
	host := ctx.Server
	if u, err := url.Parse(ctx.Server); err == nil && u.Host != "" {
		host = u.Host
	}
	if ctx.Name == "" {
		return fmt.Sprintf("server %s", host)
	}
	return fmt.Sprintf("context %q (%s)", ctx.Name, host)
}

// mutationTarget is describeTarget for callers that have only the flags — the
// confirmation gates, which must name the target environment *before* a client
// is built. Resolution errors degrade to a placeholder rather than replacing
// the missing-confirmation error with a config error: the user's actual mistake
// is the absent --yes, and a broken context surfaces on the next attempt.
func mutationTarget(global *GlobalFlags) string {
	ctx, err := resolveContext(global)
	if err != nil {
		return "an unresolved target (see `ackoctl config view`)"
	}
	return describeTarget(ctx)
}

func warnWorkspaceFallback(w io.Writer, verbose bool, ws string) {
	if !verbose {
		return
	}
	fmt.Fprintf(w, "ackoctl: using workspace=%s from current context (set --workspace to override)\n", ws)
}
