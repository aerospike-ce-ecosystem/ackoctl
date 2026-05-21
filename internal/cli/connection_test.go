package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHostsTrimsAndFilters(t *testing.T) {
	got, err := sanitizeHosts([]string{" a.example ", "", "b.example"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.example", "b.example"}, got)
}

func TestSanitizeHostsRejectsAllEmpty(t *testing.T) {
	_, err := sanitizeHosts([]string{"", "  "})
	require.Error(t, err)
}

func TestSanitizeHostsNilPassthrough(t *testing.T) {
	got, err := sanitizeHosts(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// parseLabels
// ---------------------------------------------------------------------------

func TestParseLabelsValidPairs(t *testing.T) {
	got, err := parseLabels([]string{"env=prod", "team=core"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod", "team": "core"}, got)
}

func TestParseLabelsAllowsEmptyValue(t *testing.T) {
	// "key=" is a deliberate empty value, distinct from a malformed "key".
	got, err := parseLabels([]string{"key="})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key": ""}, got)
}

func TestParseLabelsRejectsMissingEquals(t *testing.T) {
	_, err := parseLabels([]string{"notakeyvalue"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestParseLabelsNilPassthrough(t *testing.T) {
	got, err := parseLabels(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// connection CLI round-trips
// ---------------------------------------------------------------------------

// runConnectionCmd mirrors runAdminCmd: builds a root command, points it at the
// httptest server via env, and forces JSON output. It owns all env setup —
// including the optional context workspace — so call sites never have to set
// ACKOCTL_* vars themselves and cannot create env-ordering coupling.
func runConnectionCmd(t *testing.T, srvURL, workspace string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	if workspace != "" {
		t.Setenv("ACKOCTL_WORKSPACE", workspace)
	}
	root.SetArgs(append([]string{"--output", "json"}, args...))
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestConnectionListRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/connections", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"c1","name":"Prod","hosts":["h1"],"port":3000,"workspaceId":"ws-1"}]`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runConnectionCmd(t, srv.URL, "", "connection", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"name": "Prod"`)
}

func TestConnectionListPropagatesContextWorkspace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ws-ctx", r.URL.Query().Get("workspace_id"),
			"list must scope to the context workspace, never fall back to all workspaces")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "ws-ctx", "connection", "list")
	require.NoError(t, err)
}

func TestConnectionListVerboseEnvWorkspaceDoesNotClaimContextFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ws-env", r.URL.Query().Get("workspace_id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runConnectionCmd(t, srv.URL, "ws-env", "--verbose", "connection", "list")
	require.NoError(t, err)
	assert.NotContains(t, stderr, "from current context")
	assert.NotContains(t, stderr, "set --workspace to override")
}

func TestConnectionListTableShowsNoteColumn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"c1","name":"Prod","hosts":["h1"],"port":3000,"note":"primary cluster"}]`))
	}))
	t.Cleanup(srv.Close)
	// Default table output exercises the table-renderer closure.
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"connection", "list"})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())
	out := stdout.String()
	assert.Contains(t, out, "NOTE")
	assert.Contains(t, out, "primary cluster")
}

func TestConnectionGetRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/connections/c1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","name":"Prod","hosts":["h1"],"port":3000}`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runConnectionCmd(t, srv.URL, "", "connection", "get", "c1")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"id": "c1"`)
}

func TestConnectionCreateRoundTrip(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/connections", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"Prod","hosts":["h1","h2"],"port":3000}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "",
		"connection", "create", "--name", "Prod",
		"--host", "h1", "--host", "h2", "--port", "3000",
	)
	require.NoError(t, err)
	assert.Equal(t, "Prod", body["name"])
	assert.Equal(t, []any{"h1", "h2"}, body["hosts"])
	assert.Equal(t, float64(3000), body["port"])
}

func TestConnectionCreatePropagatesContextWorkspace(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"Prod","hosts":["h1"],"port":3000,"workspaceId":"ws-ctx"}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "ws-ctx",
		"connection", "create", "--name", "Prod", "--host", "h1",
	)
	require.NoError(t, err)
	assert.Equal(t, "ws-ctx", body["workspaceId"],
		"create must scope the new connection to the context workspace when --workspace is not given")
}

func TestConnectionCreateVerboseEnvWorkspaceDoesNotClaimContextFallback(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"Prod","hosts":["h1"],"port":3000,"workspaceId":"ws-env"}`))
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runConnectionCmd(t, srv.URL, "ws-env",
		"--verbose", "connection", "create", "--name", "Prod", "--host", "h1",
	)
	require.NoError(t, err)
	assert.Equal(t, "ws-env", body["workspaceId"])
	assert.NotContains(t, stderr, "from context")
	assert.NotContains(t, stderr, "set --workspace to override")
}

func TestConnectionCreateParsesLabels(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"Prod","hosts":["h1"],"port":3000}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "",
		"connection", "create", "--name", "Prod", "--host", "h1",
		"--label", "env=prod", "--label", "team=core",
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"env": "prod", "team": "core"}, body["labels"])
}

func TestConnectionCreateRejectsInvalidLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit on a malformed --label")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "",
		"connection", "create", "--name", "Prod", "--host", "h1", "--label", "malformed",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestConnectionCreateRequiresName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --name is missing")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "", "connection", "create", "--host", "h1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestConnectionUpdateSendsOnlyChangedFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/connections/c1", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","name":"Prod","hosts":["h1"],"port":3000,"note":"updated"}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "",
		"connection", "update", "c1", "--note", "updated",
	)
	require.NoError(t, err)
	// Only the flag the user actually changed may appear in the PUT body —
	// an unset flag leaking through would silently overwrite a server-side
	// field with a zero value.
	assert.Equal(t, map[string]any{"note": "updated"}, body)
}

func TestConnectionDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runConnectionCmd(t, srv.URL, "", "connection", "delete", "c1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestConnectionDeleteWithYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/connections/c1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runConnectionCmd(t, srv.URL, "", "connection", "delete", "c1", "--yes")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted connection")
}

func TestConnectionHealthRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/connections/c1/health", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connected":true,"nodeCount":3,"namespaceCount":1,"build":"8.1.0","edition":"Community"}`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runConnectionCmd(t, srv.URL, "", "connection", "health", "c1")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"connected": true`)
}
