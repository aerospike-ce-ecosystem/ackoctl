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

// Regression: cluster-manager's CreateNamespaceRequest body uses the JSON
// key `name` for the namespace. An earlier build of this command sent
// `namespace` instead, which the server rejected with HTTP 422
// ({"loc":["body","name"],"msg":"Field required"}).
func TestClusterConfigureNamespaceSendsNameField(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{
		"cluster", "configure-namespace", "conn-1",
		"--name", "test",
		"--param", "stop-writes-pct=90",
	})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())

	// The namespace identifier must arrive as `name`, not `namespace`.
	assert.Equal(t, "test", got["name"], "body must use the `name` key for the namespace")
	_, hasLegacy := got["namespace"]
	assert.False(t, hasLegacy, "must not send the legacy `namespace` key")
	// Runtime params are passed through verbatim.
	assert.Equal(t, "90", got["stop-writes-pct"])
}

// Regression: --param is registered with StringArrayVar (not StringSliceVar)
// so a value containing commas — e.g. a storage-engine device list — survives
// intact. StringSliceVar would split on the comma, sending only the first
// fragment and dropping the rest, or failing the key=value check entirely.
func TestClusterConfigureNamespacePreservesCommasInParamValue(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{
		"cluster", "configure-namespace", "conn-1",
		"--name", "test",
		"--param", "storage-engine=device,/dev/sda",
		"--param", "stop-writes-pct=90",
	})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())

	// The comma-containing value must arrive whole — not truncated at the comma.
	assert.Equal(t, "device,/dev/sda", got["storage-engine"],
		"comma-containing --param value must not be split")
	assert.Equal(t, "90", got["stop-writes-pct"])
}

func TestClusterConfigureNamespaceRejectsReservedParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when client guard rejects the input")
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	// --param name=... must be refused — `name` is reserved for --name.
	root.SetArgs([]string{
		"cluster", "configure-namespace", "conn-1",
		"--name", "test",
		"--param", "name=other",
	})
	root.SetContext(context.Background())
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name=... is reserved")
}

func TestClusterConfigureNamespaceRejectsEmptyParamKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when client guard rejects the input")
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	// "=90" splits into an empty key with ok=true; it must be rejected so an
	// unnamed entry never lands in the request body.
	root.SetArgs([]string{
		"cluster", "configure-namespace", "conn-1",
		"--name", "test",
		"--param", "=90",
	})
	root.SetContext(context.Background())
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key must not be empty")
}

func TestClusterConfigureNamespaceRequiresAtLeastOneParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when no --param is supplied")
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	// No --param: the request would carry only {"name": ...}, a no-op the
	// server cannot act on. The guard must reject it before any HTTP call.
	root.SetArgs([]string{
		"cluster", "configure-namespace", "conn-1",
		"--name", "test",
	})
	root.SetContext(context.Background())
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one --param")
}
