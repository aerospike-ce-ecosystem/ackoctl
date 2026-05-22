package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runRoot(t *testing.T, configPath string, args ...string) (string, string, error) {
	t.Helper()
	cmd := NewRootCmd()
	full := append([]string{"--config", configPath}, args...)
	cmd.SetArgs(full)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestConfigLifecycle(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	out, _, err := runRoot(t, cfgPath, "config", "set-context", "kind-local",
		"--server", "http://localhost:8000/api",
		"--workspace-id", "default",
	)
	require.NoError(t, err)
	assert.Contains(t, out, "saved")

	out, _, err = runRoot(t, cfgPath, "config", "current-context")
	require.NoError(t, err)
	assert.Contains(t, out, "kind-local")

	out, _, err = runRoot(t, cfgPath, "config", "set-context", "prod",
		"--server", "https://acm.example.com/api",
		"--token", "tok",
	)
	require.NoError(t, err)
	assert.Contains(t, out, "saved")

	out, _, err = runRoot(t, cfgPath, "config", "use-context", "prod")
	require.NoError(t, err)
	assert.Contains(t, out, "prod")

	out, _, err = runRoot(t, cfgPath, "config", "view", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "kind-local")
	assert.Contains(t, out, "prod")
	assert.Contains(t, out, `"current-context": "prod"`)

	_, _, err = runRoot(t, cfgPath, "config", "delete-context", "prod")
	require.NoError(t, err)

	out, _, err = runRoot(t, cfgPath, "config", "view", "-o", "json")
	require.NoError(t, err)
	assert.NotContains(t, out, `"name": "prod"`)
}

func TestConfigViewRedactsToken(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	_, _, err := runRoot(t, cfgPath, "config", "set-context", "prod",
		"--server", "https://acm.example.com/api",
		"--token", "super-secret-token",
	)
	require.NoError(t, err)

	// json and yaml views must never print the real token.
	for _, format := range []string{"json", "yaml"} {
		out, _, err := runRoot(t, cfgPath, "config", "view", "-o", format)
		require.NoError(t, err)
		assert.NotContains(t, out, "super-secret-token", "%s view leaked the token", format)
		assert.Contains(t, out, "***", "%s view should show the redaction placeholder", format)
	}

	// The on-disk config file must still carry the real token.
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "super-secret-token",
		"redaction must not alter the persisted config file")
}

func TestSetContextRequiresServerOnCreate(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	_, _, err := runRoot(t, cfgPath, "config", "set-context", "noserver")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--server")
}

func TestVersionCommand(t *testing.T) {
	out, _, err := runRoot(t, filepath.Join(t.TempDir(), "x.yaml"), "version", "--short")
	require.NoError(t, err)
	assert.Equal(t, "dev\n", out)
}
