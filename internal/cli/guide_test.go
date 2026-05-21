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

// runGuideCmd wires the guide command against an httptest server and captures
// stdout/stderr. --server / --token come from env so resolveContext does not
// read ~/.ackoctl/config.yaml.
func runGuideCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(args)
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestGuideGetPrintsMarkdownByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No --workspace, no context → fall back to the built-in default.
		assert.Equal(t, "/v1/guides/ws-default/data-plane", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceId":"ws-default","guideType":"data-plane","title":"DP","content":"# Data-plane policy\n\nTTL <= 7d","createdAt":"t","updatedAt":"t"}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runGuideCmd(t, srv.URL, "guide", "get", "data-plane")
	require.NoError(t, err)
	// Default output is the raw Markdown body, not the JSON envelope.
	assert.Contains(t, stdout, "# Data-plane policy")
	assert.Contains(t, stdout, "TTL <= 7d")
	assert.NotContains(t, stdout, `"guideType"`)
}

func TestGuideGetJSONOutputsStructured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/guides/ws-team-a/control-plane", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceId":"ws-team-a","guideType":"control-plane","title":"CP","content":"body","createdAt":"t","updatedAt":"t"}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runGuideCmd(t, srv.URL,
		"--workspace", "ws-team-a", "--output", "json", "guide", "get", "control-plane")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"guideType": "control-plane"`)
}

func TestGuideGetRejectsInvalidType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit for an invalid guide type")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runGuideCmd(t, srv.URL, "guide", "get", "bogus-plane")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid guide type")
}

func TestGuideListRendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/guides/ws-default", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"guides":[{"workspaceId":"ws-default","guideType":"data-plane","title":"DP policy","content":"x","createdAt":"t","updatedAt":"2026-05-21","updatedBy":"alice"}]}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runGuideCmd(t, srv.URL, "guide", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "TYPE")
	assert.Contains(t, stdout, "data-plane")
	assert.Contains(t, stdout, "alice")
}

func TestGuideGetUsesWorkspaceFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/guides/ws-custom/data-plane", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceId":"ws-custom","guideType":"data-plane","title":"DP","content":"x","createdAt":"t","updatedAt":"t"}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runGuideCmd(t, srv.URL, "--workspace", "ws-custom", "guide", "get", "data-plane")
	require.NoError(t, err)
}
