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

// runInfoCmd mirrors runNoteCmd: wires the root command against an httptest
// server, isolates HOME so the config loader can't leak, and forces JSON
// output unless the caller passes their own “--output“.
func runInfoCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(append([]string{"--output", "json"}, args...))
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestInfoCommandFlagIsRepeatable(t *testing.T) {
	// --command must accept multiple occurrences AND must NOT comma-split
	// (asinfo verbs like ``set-config:context=...`` legitimately contain
	// commas). This is why the flag uses StringArrayVar, not StringSliceVar.
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/clusters/conn-1/info", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"command":"build","node":"BB9020011AC4202","output":"8.1.0.0"},
			{"command":"status","node":"BB9020011AC4202","output":"ok"}
		]}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runInfoCmd(t, srv.URL,
		"info", "conn-1",
		"--command", "build",
		"--command", "status",
	)
	require.NoError(t, err)
	cmds, ok := body["commands"].([]any)
	require.True(t, ok, "commands must serialize as JSON array")
	require.Len(t, cmds, 2)
	assert.Equal(t, "build", cmds[0])
	assert.Equal(t, "status", cmds[1])
	assert.Contains(t, stdout, `"output": "8.1.0.0"`)
}

func TestInfoDefaultsReadOnlyTrue(t *testing.T) {
	// Without --allow-write the client must send readOnly=true so the server
	// enforces the asinfo whitelist. This is the safety default that keeps
	// users from accidentally issuing set-config: writes.
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runInfoCmd(t, srv.URL, "info", "conn-1", "--command", "build")
	require.NoError(t, err)
	assert.Equal(t, true, body["readOnly"], "readOnly must default to true on the wire")
}

func TestInfoAllowWriteFlipsReadOnly(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runInfoCmd(t, srv.URL,
		"info", "conn-1",
		"--command", "set-config:context=service;proto-fd-max=20000",
		"--allow-write", "--yes",
	)
	require.NoError(t, err)
	assert.Equal(t, false, body["readOnly"], "--allow-write must send readOnly=false")
	// Comma-bearing verbs (none here, but we also verify the colon/equals
	// content survived the flag parser unchanged).
	cmds := body["commands"].([]any)
	require.Len(t, cmds, 1)
	assert.Equal(t, "set-config:context=service;proto-fd-max=20000", cmds[0])
}

func TestInfoNodeFlagTargetsSingleNode(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"command":"build","node":"BB9020011AC4202","output":"8.1.0.0"}]}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runInfoCmd(t, srv.URL,
		"info", "conn-1",
		"--command", "build",
		"--node", "BB9020011AC4202",
	)
	require.NoError(t, err)
	assert.Equal(t, "BB9020011AC4202", body["node"])
}

func TestInfoRequiresCommandFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be hit when --command is missing")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runInfoCmd(t, srv.URL, "info", "conn-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestInfoRejectsEmptyCommandValue(t *testing.T) {
	// MarkFlagRequired only checks --command was supplied. An empty value
	// (`--command ""`) must still be rejected before any server round-trip.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be hit when --command is empty")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runInfoCmd(t, srv.URL, "info", "conn-1", "--command", "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestInfoTableOutputRendersHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"command":"build","node":"BB9020011AC4202","output":"8.1.0.0"},
			{"command":"build","node":"BB9020011AC4203","output":"","error":"timeout"}
		]}`))
	}))
	t.Cleanup(srv.Close)

	// Default output is table; do not pass --output.
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"info", "conn-1", "--command", "build"})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())
	out := stdout.String()
	assert.Contains(t, out, "NODE")
	assert.Contains(t, out, "COMMAND")
	assert.Contains(t, out, "OUTPUT")
	assert.Contains(t, out, "ERROR")
	assert.Contains(t, out, "8.1.0.0")
	assert.Contains(t, out, "timeout")
}

// TestInfoAllowWriteRequiresYes pins the confirmation gate on the one info
// path that mutates a live cluster. Before this gate, `info --allow-write
// --command 'set-config:...'` needed no confirmation while deleting a single
// secondary index did: cluster-manager skips its read-only verb whitelist
// entirely when readOnly=false, so any write verb reached the cluster.
func TestInfoAllowWriteRequiresYes(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantErr      string // non-empty => expect this substring and no HTTP call
		wantCalled   bool
		wantReadOnly bool
	}{
		{
			name:    "--allow-write without --yes is refused",
			args:    []string{"--allow-write"},
			wantErr: "confirmation required (--yes)",
		},
		{
			name:         "--allow-write with --yes proceeds",
			args:         []string{"--allow-write", "--yes"},
			wantCalled:   true,
			wantReadOnly: false,
		},
		{
			name:         "--allow-write with -y proceeds",
			args:         []string{"--allow-write", "-y"},
			wantCalled:   true,
			wantReadOnly: false,
		},
		{
			// The read-only default is not destructive, so it must stay
			// confirmation-free — the gate keys on impact, not on the command.
			name:         "read-only run needs no confirmation",
			args:         nil,
			wantCalled:   true,
			wantReadOnly: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[]}`))
			}))
			t.Cleanup(srv.Close)

			args := append([]string{
				"info", "conn-1",
				"--command", "set-config:context=service;proto-fd-max=20000",
			}, tc.args...)
			_, _, err := runInfoCmd(t, srv.URL, args...)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				// The refusal names the environment the operator was about to
				// mutate, so a wrong-context mistake is visible immediately.
				assert.Contains(t, err.Error(), "127.0.0.1")
				assert.False(t, called, "gate must refuse before any HTTP call")
				return
			}
			require.NoError(t, err)
			assert.True(t, called, "confirmed run must reach the server")
			assert.Equal(t, tc.wantReadOnly, body["readOnly"])
		})
	}
}
