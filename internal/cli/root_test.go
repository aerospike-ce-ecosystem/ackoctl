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

// An invalid -o value must be rejected in the root PersistentPreRunE, BEFORE
// any subcommand RunE builds a client or contacts the server. Otherwise a
// non-idempotent mutation (e.g. `record put ... -o xml`) would run server-side
// and only THEN exit 1 on the unknown format, misleading the user into
// retrying the write.
func TestInvalidOutputFormatFailsBeforeServerIsContacted(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hit = true
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	t.Setenv("ACKOCTL_NO_VERSION_CHECK", "1")
	root.SetArgs([]string{"k8s", "cluster", "list", "-o", "bogus"})
	root.SetContext(context.Background())

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output format")
	assert.False(t, hit, "server must never be contacted when -o is invalid")
}

// A valid -o value (and the default) must pass the pre-run validation so
// normal commands still run.
func TestValidOutputFormatPassesPreRunValidation(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	t.Setenv("ACKOCTL_NO_VERSION_CHECK", "1")
	root.SetArgs([]string{"k8s", "cluster", "list", "-o", "json"})
	root.SetContext(context.Background())

	require.NoError(t, root.Execute())
	assert.True(t, hit, "a valid -o value must not block the command")
}
