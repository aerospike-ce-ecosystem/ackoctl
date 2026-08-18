package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
)

// --workspace is a persistent flag, so cobra hands it to every command whether
// or not the command can do anything with it. Until #90 that meant 52 of the
// 57 runnable commands parsed the flag into GlobalFlags and never read it
// again: an operator who scoped a command got no error, no warning, and no
// scoping. The annotations below make each command state which of the three
// possible answers applies, and the check in NewRootCmd's PersistentPreRunE
// enforces the declaration.
//
// The check is default-deny: a command carrying no annotation rejects
// --workspace. A new command therefore cannot re-create the accept-and-ignore
// state by omission — it either declares support or it refuses the flag.
const (
	annoWorkspace       = "ackoctl.io/workspace"
	annoWorkspaceReason = "ackoctl.io/workspace-unsupported-reason"
)

const (
	// wsScoped: cluster-manager accepts a workspace on this endpoint and
	// scopes the operation server-side.
	wsScoped = "scoped"

	// wsConnGuarded: the command operates on one connection profile, named by
	// args[0]. cluster-manager has no workspace parameter on those endpoints —
	// it derives the workspace from the stored profile — but it does report
	// the profile's workspace on GET /connections/{id}, so ackoctl can resolve
	// the target first and refuse when it is in a different workspace.
	wsConnGuarded = "conn-guarded"

	// wsUnsupported: nothing on this command addresses a workspace-scoped
	// resource, so --workspace is rejected at parse time.
	wsUnsupported = "unsupported"
)

// k8sWorkspaceLabel is the CR metadata label a workspace is stamped onto. It
// aliases the client-side constant so the help text and the label selector on
// the wire can never disagree.
const k8sWorkspaceLabel = client.K8sWorkspaceLabel

// markWorkspace records how cmd handles --workspace. reason is required for
// wsUnsupported and is quoted back to the user in the rejection error.
func markWorkspace(cmd *cobra.Command, mode, reason string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annoWorkspace] = mode
	if reason != "" {
		cmd.Annotations[annoWorkspaceReason] = reason
	}
	return cmd
}

// wsScopedCmd marks a command that sends the workspace to cluster-manager.
func wsScopedCmd(cmd *cobra.Command) *cobra.Command {
	return markWorkspace(cmd, wsScoped, "")
}

// wsGuardedCmd marks a connection-scoped command. Its RunE must obtain the
// client through newConnClient (or check the fetched profile itself) so the
// declaration and the behaviour cannot drift apart; TestGuardedCommandsRefuse
// executes every command carrying this annotation to prove they have not.
func wsGuardedCmd(cmd *cobra.Command) *cobra.Command {
	return markWorkspace(cmd, wsConnGuarded, "")
}

// wsUnsupportedCmd marks a command that rejects --workspace, with the reason
// the user sees.
func wsUnsupportedCmd(cmd *cobra.Command, reason string) *cobra.Command {
	return markWorkspace(cmd, wsUnsupported, reason)
}

// reasonNoServer is the rejection reason shared by every command that never
// contacts cluster-manager.
const reasonNoServer = "it does not contact cluster-manager"

// reasonSetContext points at the flag that does what a user reaching for
// --workspace on `config set-context` almost certainly meant.
const reasonSetContext = "it does not contact cluster-manager; " +
	"use --workspace-id to give this context a default workspace"

// reasonNoK8sWorkspace explains why the per-cluster k8s commands cannot honour
// the flag. Unlike connection profiles, an AerospikeCluster CR's workspace
// never reaches the client: K8sClusterSummary and K8sClusterDetail both drop
// the metadata labels the workspace is stamped on, so there is nothing for
// ackoctl to compare against.
const reasonNoK8sWorkspace = "cluster-manager returns no workspace for a single cluster — " +
	"K8sClusterSummary and K8sClusterDetail both omit the CR's " + k8sWorkspaceLabel + " label, " +
	"so ackoctl cannot verify which workspace this cluster belongs to"

// checkWorkspaceSupported rejects --workspace on commands that cannot honour
// it. It runs from the root PersistentPreRunE, which cobra calls with the
// command actually being executed.
//
// Only an explicit --workspace is rejected. $ACKOCTL_WORKSPACE and a context's
// workspace-id are ambient configuration — a user who exports the variable
// once should not have `ackoctl version` start failing — so those are ignored
// here and simply do not apply to an unsupported command.
func checkWorkspaceSupported(c *cobra.Command) error {
	if !c.Flags().Changed("workspace") {
		return nil
	}
	switch c.Annotations[annoWorkspace] {
	case wsScoped, wsConnGuarded:
		return nil
	default:
		return unsupportedWorkspaceError(c)
	}
}

func unsupportedWorkspaceError(c *cobra.Command) error {
	reason := c.Annotations[annoWorkspaceReason]
	if reason == "" {
		reason = "it does not address a workspace-scoped resource"
	}
	return fmt.Errorf(`--workspace is not supported by %q: %s

Commands that honour --workspace:
  ackoctl connection list|create        cluster-manager filters/assigns by workspace
  ackoctl guide list|get                guides are addressed by workspace
  ackoctl k8s cluster list              filters CRs by their %s label
  ackoctl <noun> <verb> CONN_ID ...     refuses when CONN_ID is in another workspace

To give a context a default workspace instead:
  ackoctl config set-context NAME --workspace-id ID`,
		c.CommandPath(), reason, k8sWorkspaceLabel)
}

// workspaceSource names where the effective workspace came from, so a refusal
// points at the thing the user has to change. The precedence mirrors
// config.Resolve: flag, then environment, then the context on disk.
func workspaceSource(global *GlobalFlags) string {
	switch {
	case global.WorkspaceExplicit:
		return "--workspace"
	case global.WorkspaceEnvExplicit:
		return "$ACKOCTL_WORKSPACE"
	default:
		return "the current context's workspace-id"
	}
}

// newConnClient builds a client and refuses to hand it back when connID is not
// in the effective workspace.
//
// cluster-manager exposes no workspace parameter on /records, /sets, /query,
// /indexes, /udfs, /admin, /notes or /clusters: those endpoints take a
// connection id and derive the workspace from the stored profile. Resolving
// the profile first is therefore the only way ackoctl can honour --workspace
// on them. GET /connections/{id} runs the same default-deny ACL as the
// operation itself (dependencies._get_verified_connection), so the extra call
// can never show the caller a profile the operation would have refused.
func newConnClient(cmd *cobra.Command, global *GlobalFlags, connID string) (*client.BaseClient, error) {
	c, err := newClient(cmd, global)
	if err != nil {
		return nil, err
	}
	if c.Workspace == "" {
		return c, nil
	}
	conn, err := c.GetConnection(cmd.Context(), connID)
	if err != nil {
		return nil, err
	}
	if err := checkConnWorkspace(cmd, global, connID, c.Workspace, conn.WorkspaceID); err != nil {
		return nil, err
	}
	return c, nil
}

// checkConnWorkspace compares a resolved connection's workspace against the
// one the user asked for. want is never empty — callers skip the check when no
// workspace is in effect.
func checkConnWorkspace(cmd *cobra.Command, global *GlobalFlags, connID, want, got string) error {
	switch got {
	case want:
		return nil
	case "":
		// cluster-manager treats a profile with no workspaceId as shared with
		// every authenticated caller (_get_verified_connection returns before
		// the ACL check for it), so there is no workspace to compare against.
		// Refusing would make --workspace unusable against any profile created
		// before workspaces existed; saying so on stderr keeps the operator
		// from believing a scope was applied.
		fmt.Fprintf(cmd.ErrOrStderr(),
			"ackoctl: connection %q reports no workspace; cluster-manager treats it as shared, so workspace %q (from %s) could not be enforced\n",
			connID, want, workspaceSource(global))
		return nil
	default:
		return fmt.Errorf("connection %q is in workspace %q, not %q (from %s): refusing to run %q against it",
			connID, got, want, workspaceSource(global), cmd.CommandPath())
	}
}
