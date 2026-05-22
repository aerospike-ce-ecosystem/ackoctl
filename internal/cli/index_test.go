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

// runIndexCmd mirrors runInfoCmd: wires the root command against an httptest
// server, isolates HOME so the config loader can't leak, and forces JSON
// output.
func runIndexCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
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

func TestIndexCreateRejectsUnknownType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when --type is invalid")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runIndexCmd(t, srv.URL,
		"index", "create", "conn-1",
		"--namespace", "test", "--set", "s", "--bin", "b",
		"--name", "idx", "--type", "geo",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "numeric|string|geo2dsphere")
}

func TestIndexCreateAcceptsValidType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"test","set":"s","name":"idx","bin":"b","type":"numeric","state":"RW"}`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runIndexCmd(t, srv.URL,
		"index", "create", "conn-1",
		"--namespace", "test", "--set", "s", "--bin", "b",
		"--name", "idx", "--type", "numeric",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"name": "idx"`)
}
