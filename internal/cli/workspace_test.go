package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runWorkspaceCmd drives the real cobra tree against srvURL with HOME
// redirected, so the on-disk config can never leak a workspace into an
// assertion about flag handling.
func runWorkspaceCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	t.Setenv("ACKOCTL_NO_VERSION_CHECK", "1")
	root.SetArgs(args)
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// runnableCommands returns every leaf command in the tree, keyed by command
// path. cobra generates `help` and the `__complete*` hidden commands on
// Execute rather than in NewRootCmd; they are skipped by name so the walk
// describes only commands this repo owns.
func runnableCommands(t *testing.T) map[string]*cobra.Command {
	t.Helper()
	out := map[string]*cobra.Command{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			walk(sub)
		}
		if !c.Runnable() || c.Name() == "help" || strings.HasPrefix(c.Name(), "__") {
			return
		}
		out[c.CommandPath()] = c
	}
	walk(NewRootCmd())
	return out
}

func commandsWithMode(t *testing.T, mode string) map[string]*cobra.Command {
	t.Helper()
	out := map[string]*cobra.Command{}
	for path, c := range runnableCommands(t) {
		if c.Annotations[annoWorkspace] == mode {
			out[path] = c
		}
	}
	return out
}

// Every runnable command must state how it handles --workspace. This is the
// test that keeps #90 fixed: a new command that neither scopes by workspace
// nor rejects the flag fails here, instead of shipping as another silent drop.
func TestEveryCommandDeclaresWorkspaceHandling(t *testing.T) {
	cmds := runnableCommands(t)
	require.NotEmpty(t, cmds)
	for path, c := range cmds {
		mode := c.Annotations[annoWorkspace]
		assert.Contains(t, []string{wsScoped, wsConnGuarded, wsUnsupported}, mode,
			"%s must declare --workspace handling via wsScopedCmd / wsGuardedCmd / wsUnsupportedCmd", path)
		if mode == wsUnsupported {
			assert.NotEmpty(t, c.Annotations[annoWorkspaceReason],
				"%s rejects --workspace and must say why", path)
		}
	}
}

// The declared split is asserted verbatim so that moving a command between
// buckets is a deliberate, reviewable edit rather than a side effect.
func TestWorkspaceHandlingMatchesDeclaredSplit(t *testing.T) {
	want := map[string]string{
		"ackoctl connection list":   wsScoped,
		"ackoctl connection create": wsScoped,
		"ackoctl guide list":        wsScoped,
		"ackoctl guide get":         wsScoped,
		"ackoctl k8s cluster list":  wsScoped,

		"ackoctl cluster info":                wsConnGuarded,
		"ackoctl cluster configure-namespace": wsConnGuarded,
		"ackoctl admin user list":             wsConnGuarded,
		"ackoctl admin user create":           wsConnGuarded,
		"ackoctl admin user passwd":           wsConnGuarded,
		"ackoctl admin user delete":           wsConnGuarded,
		"ackoctl admin role list":             wsConnGuarded,
		"ackoctl admin role create":           wsConnGuarded,
		"ackoctl admin role delete":           wsConnGuarded,
		"ackoctl connection get":              wsConnGuarded,
		"ackoctl connection update":           wsConnGuarded,
		"ackoctl connection delete":           wsConnGuarded,
		"ackoctl connection health":           wsConnGuarded,
		"ackoctl index list":                  wsConnGuarded,
		"ackoctl index create":                wsConnGuarded,
		"ackoctl index delete":                wsConnGuarded,
		"ackoctl info":                        wsConnGuarded,
		"ackoctl note set list":               wsConnGuarded,
		"ackoctl note set update":             wsConnGuarded,
		"ackoctl note set delete":             wsConnGuarded,
		"ackoctl note record list":            wsConnGuarded,
		"ackoctl note record update":          wsConnGuarded,
		"ackoctl note record delete":          wsConnGuarded,
		"ackoctl query exec":                  wsConnGuarded,
		"ackoctl record list":                 wsConnGuarded,
		"ackoctl record get":                  wsConnGuarded,
		"ackoctl record put":                  wsConnGuarded,
		"ackoctl record delete":               wsConnGuarded,
		"ackoctl record delete-bin":           wsConnGuarded,
		"ackoctl record query":                wsConnGuarded,
		"ackoctl set list":                    wsConnGuarded,
		"ackoctl set truncate":                wsConnGuarded,
		"ackoctl udf list":                    wsConnGuarded,
		"ackoctl udf upload":                  wsConnGuarded,
		"ackoctl udf remove":                  wsConnGuarded,

		"ackoctl version":                wsUnsupported,
		"ackoctl upgrade":                wsUnsupported,
		"ackoctl completion bash":        wsUnsupported,
		"ackoctl completion zsh":         wsUnsupported,
		"ackoctl completion fish":        wsUnsupported,
		"ackoctl completion powershell":  wsUnsupported,
		"ackoctl config view":            wsUnsupported,
		"ackoctl config set-context":     wsUnsupported,
		"ackoctl config use-context":     wsUnsupported,
		"ackoctl config current-context": wsUnsupported,
		"ackoctl config delete-context":  wsUnsupported,
		"ackoctl k8s cluster get":        wsUnsupported,
		"ackoctl k8s cluster pods":       wsUnsupported,
		"ackoctl k8s cluster logs":       wsUnsupported,
		"ackoctl k8s cluster events":     wsUnsupported,
		"ackoctl k8s cluster reconcile":  wsUnsupported,
		"ackoctl k8s cluster scale":      wsUnsupported,
	}

	got := map[string]string{}
	for path, c := range runnableCommands(t) {
		got[path] = c.Annotations[annoWorkspace]
	}
	assert.Equal(t, want, got)
}

// positionalArgs supplies the positional arguments a command needs to get past
// cobra's Args validation, which runs ahead of the --workspace check. Required
// *flags* are deliberately not supplied: ValidateRequiredFlags runs after
// PersistentPreRunE, so the rejection must fire without them.
var positionalArgs = map[string][]string{
	"ackoctl k8s cluster get":       {"ns/c1"},
	"ackoctl k8s cluster pods":      {"ns/c1"},
	"ackoctl k8s cluster logs":      {"ns/c1"},
	"ackoctl k8s cluster events":    {"ns/c1"},
	"ackoctl k8s cluster reconcile": {"ns/c1"},
	"ackoctl k8s cluster scale":     {"ns/c1"},
	"ackoctl config set-context":    {"ctx"},
	"ackoctl config use-context":    {"ctx"},
	"ackoctl config delete-context": {"ctx"},
}

// --workspace on a command that cannot honour it must fail before anything is
// sent, and the error must name what the user can do instead.
func TestUnsupportedCommandsRejectWorkspaceFlag(t *testing.T) {
	for path, c := range commandsWithMode(t, wsUnsupported) {
		t.Run(path, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Errorf("%s contacted the server despite an unsupported --workspace", path)
			}))
			t.Cleanup(srv.Close)

			argv := strings.Fields(strings.TrimPrefix(path, "ackoctl "))
			argv = append(argv, positionalArgs[path]...)
			argv = append(argv, "--workspace", "ws-a")
			_, _, err := runWorkspaceCmd(t, srv.URL, argv...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "--workspace is not supported by")
			assert.Contains(t, err.Error(), path)
			assert.Contains(t, err.Error(), c.Annotations[annoWorkspaceReason])
			assert.Contains(t, err.Error(), "ackoctl connection list|create",
				"the refusal must point at the commands that do honour --workspace")
		})
	}
}

// The rejection is scoped to an explicit flag. $ACKOCTL_WORKSPACE and a
// context's workspace-id are ambient configuration — exporting the variable
// once must not start breaking `ackoctl version`.
func TestAmbientWorkspaceDoesNotRejectUnsupportedCommands(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	t.Setenv("ACKOCTL_WORKSPACE", "ws-a")
	stdout, _, err := runWorkspaceCmd(t, srv.URL, "version", "--short")
	require.NoError(t, err)
	assert.Contains(t, stdout, "dev")
}

// requiredFlagValues supplies argv values for guarded commands whose own
// validation runs before the workspace guard. Without a usable value those
// commands would fail early and the guard assertion below would pass
// vacuously. Keyed by "<command path> --<flag>".
var requiredFlagValues = map[string]string{
	"ackoctl index create --type": "string",
	"ackoctl record put --bins":   `{"age":1}`,
}

// extraGuardedArgs covers arguments cobra does not mark with the required-flag
// annotation — one-of flag groups, and flags a command validates itself before
// building a client.
var extraGuardedArgs = map[string][]string{
	"ackoctl admin user create":           {"--password", "pw"},
	"ackoctl admin user passwd":           {"--password", "pw"},
	"ackoctl cluster configure-namespace": {"--param", "replicationFactor=2"},
}

// guardedArgv builds a minimal invocation of a guarded command: the connection
// id as the only positional argument, a value for every required flag, and
// --yes wherever the command defines a confirmation gate.
func guardedArgv(t *testing.T, c *cobra.Command, connID string) []string {
	t.Helper()
	argv := append(strings.Fields(strings.TrimPrefix(c.CommandPath(), "ackoctl ")), connID)
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "yes" {
			argv = append(argv, "--yes")
			return
		}
		anns, ok := f.Annotations[cobra.BashCompOneRequiredFlag]
		if !ok || len(anns) == 0 || anns[0] != "true" {
			return
		}
		value := placeholderFlagValue(t, f)
		if override, ok := requiredFlagValues[c.CommandPath()+" --"+f.Name]; ok {
			value = override
		}
		argv = append(argv, "--"+f.Name, value)
	})
	return append(argv, extraGuardedArgs[c.CommandPath()]...)
}

func placeholderFlagValue(t *testing.T, f *pflag.Flag) string {
	t.Helper()
	switch f.Value.Type() {
	case "int":
		return "1"
	case "bool":
		return "true"
	case "duration":
		return "1s"
	default:
		if f.Name == "file" {
			// udf upload reads the module off disk before any request.
			path := filepath.Join(t.TempDir(), "mod.lua")
			require.NoError(t, os.WriteFile(path, []byte("function f() end\n"), 0o600))
			return path
		}
		return "x"
	}
}

// The core regression test for #90. Every command declared conn-guarded is
// executed for real against a server that reports the target connection in a
// different workspace; each must refuse, and none may send anything beyond the
// GET that resolves the connection. A guarded command whose RunE forgot to
// route through newConnClient fails here on both counts.
func TestGuardedCommandsRefuseConnectionInAnotherWorkspace(t *testing.T) {
	for path, c := range commandsWithMode(t, wsConnGuarded) {
		t.Run(path, func(t *testing.T) {
			var (
				mu    sync.Mutex
				paths []string
			)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.Method+" "+r.URL.Path)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"c1","name":"C1","hosts":["h"],"port":3000,"workspaceId":"ws-other"}`))
			}))
			t.Cleanup(srv.Close)

			argv := append(guardedArgv(t, c, "c1"), "--workspace", "ws-mine")
			_, _, err := runWorkspaceCmd(t, srv.URL, argv...)

			require.Error(t, err, "%s must refuse a connection in another workspace", path)
			assert.Contains(t, err.Error(), `connection "c1" is in workspace "ws-other", not "ws-mine"`)
			assert.Contains(t, err.Error(), "--workspace")

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, []string{"GET /v1/connections/c1"}, paths,
				"%s must resolve the connection and stop; it must not reach its own endpoint", path)
		})
	}
}

// The mirror image: a matching workspace must not block the command. Asserted
// on one representative of each shape (read, mutation, connection-addressed)
// rather than the whole set, because a passing run has to reach each command's
// real endpoint with a plausible response body.
func TestGuardedCommandsProceedWhenWorkspaceMatches(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"record get", []string{"record", "get", "c1", "--namespace", "test", "--set", "s", "--pk", "k"},
			"GET /v1/records/c1/detail"},
		{"set truncate", []string{"set", "truncate", "c1", "--namespace", "test", "--set", "s", "--yes"},
			"POST /v1/sets/c1/test/s/truncate"},
		{"connection health", []string{"connection", "health", "c1"},
			"GET /v1/connections/c1/health"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				mu    sync.Mutex
				paths []string
			)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.Method+" "+r.URL.Path)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/connections/c1" {
					_, _ = w.Write([]byte(`{"id":"c1","name":"C1","hosts":["h"],"port":3000,"workspaceId":"ws-mine"}`))
					return
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)

			_, _, err := runWorkspaceCmd(t, srv.URL, append(tc.argv, "--workspace", "ws-mine")...)
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, []string{"GET /v1/connections/c1", tc.want}, paths)
		})
	}
}

// A profile with no workspaceId is shared across every caller server-side, so
// there is nothing to compare and the command proceeds — but it must say so
// rather than leave the operator believing a scope was applied.
func TestGuardWarnsWhenConnectionReportsNoWorkspace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/connections/c1" {
			_, _ = w.Write([]byte(`{"id":"c1","name":"C1","hosts":["h"],"port":3000}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	_, stderr, err := runWorkspaceCmd(t, srv.URL,
		"record", "get", "c1", "--namespace", "test", "--set", "s", "--pk", "k",
		"--workspace", "ws-mine")
	require.NoError(t, err)
	assert.Contains(t, stderr, `connection "c1" reports no workspace`)
	assert.Contains(t, stderr, `workspace "ws-mine" (from --workspace) could not be enforced`)
}

// The guard reports where the workspace came from, so a refusal triggered by a
// stale context or an exported variable points at the right thing to change.
func TestGuardNamesTheWorkspaceSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","name":"C1","hosts":["h"],"port":3000,"workspaceId":"ws-other"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("ACKOCTL_WORKSPACE", "ws-env")
	_, _, err := runWorkspaceCmd(t, srv.URL, "cluster", "info", "c1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `not "ws-env" (from $ACKOCTL_WORKSPACE)`)
}

// With no workspace in effect the guard must stay out of the way entirely —
// no extra round trip, no behaviour change for users who never set one.
func TestNoWorkspaceMeansNoGuardRequest(t *testing.T) {
	var (
		mu    sync.Mutex
		paths []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runWorkspaceCmd(t, srv.URL,
		"record", "get", "c1", "--namespace", "test", "--set", "s", "--pk", "k")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"/v1/records/c1/detail"}, paths)
}

// k8s cluster list has no workspace parameter server-side; the workspace has
// to reach cluster-manager as a label selector over the CR label, or the
// filter silently does nothing.
func TestK8sClusterListScopesByWorkspaceLabel(t *testing.T) {
	var selector string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/k8s/clusters", r.URL.Path)
		selector = r.URL.Query().Get("labelSelector")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runWorkspaceCmd(t, srv.URL, "k8s", "cluster", "list", "--workspace", "ws-team-a")
	require.NoError(t, err)
	assert.Equal(t, "acm.aerospike.com/workspace=ws-team-a", selector)
}

func TestK8sClusterListSendsNoSelectorWithoutWorkspace(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runWorkspaceCmd(t, srv.URL, "k8s", "cluster", "list")
	require.NoError(t, err)
	assert.Empty(t, query)
}

// The three paths that already transmitted a workspace before #90 must keep
// doing so — the fix must not regress the commands it was built around.
func TestScopedCommandsStillTransmitWorkspace(t *testing.T) {
	t.Run("connection list", func(t *testing.T) {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Query().Get("workspace_id")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))
		t.Cleanup(srv.Close)
		_, _, err := runWorkspaceCmd(t, srv.URL, "connection", "list", "--workspace", "ws-a")
		require.NoError(t, err)
		assert.Equal(t, "ws-a", got)
	})

	t.Run("connection create", func(t *testing.T) {
		var body map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"c1","name":"n","hosts":["h"],"port":3000,"workspaceId":"ws-a"}`))
		}))
		t.Cleanup(srv.Close)
		_, _, err := runWorkspaceCmd(t, srv.URL,
			"connection", "create", "--name", "n", "--host", "h", "--workspace", "ws-a")
		require.NoError(t, err)
		assert.Equal(t, "ws-a", body["workspaceId"])
	})

	t.Run("guide get", func(t *testing.T) {
		var path string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workspaceId":"ws-a","guideType":"data-plane","title":"t","content":"c","createdAt":"t","updatedAt":"t"}`))
		}))
		t.Cleanup(srv.Close)
		_, _, err := runWorkspaceCmd(t, srv.URL, "guide", "get", "data-plane", "--workspace", "ws-a")
		require.NoError(t, err)
		assert.Equal(t, "/v1/guides/ws-a/data-plane", path)
	})
}

// The reason text for the per-cluster k8s commands is a factual claim about
// cluster-manager's response shape, so it must name the label a reader can go
// and check.
func TestK8sRejectionExplainsMissingLabel(t *testing.T) {
	require.Contains(t, reasonNoK8sWorkspace, "acm.aerospike.com/workspace")
	require.Contains(t, reasonNoK8sWorkspace,
		fmt.Sprintf("%s label", "acm.aerospike.com/workspace"))
}
