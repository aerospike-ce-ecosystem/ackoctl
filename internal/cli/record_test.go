package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRecordCmd mirrors runNoteCmd from note_test.go: drives the real cobra
// tree against an httptest server with HOME redirected so the config loader
// cannot leak global state into the assertion.
func runRecordCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
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

func TestRecordDeleteBinHitsServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/records/conn-1/test/users/alice/bins/age", r.URL.Path)
		assert.Equal(t, "string", r.URL.Query().Get("pk_type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice",
		"--bin", "age", "--pk-type", "string", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted bin")
	assert.Contains(t, stderr, "age")
}

func TestRecordDeleteBinRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice", "--bin", "age",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestRecordDeleteBinRequiresBin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call when --bin missing")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice", "--yes",
	)
	require.Error(t, err)
	// cobra reports missing required flag(s)
	assert.Contains(t, err.Error(), "required")
}
