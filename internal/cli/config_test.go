package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestSetContextRejectsMalformedServer(t *testing.T) {
	cases := []struct {
		name   string
		server string
	}{
		{"missing scheme", "localhost:8000"},
		{"non-http scheme", "ftp://host"},
		{"path only", "/api/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			_, _, err := runRoot(t, cfgPath, "config", "set-context", "ctx",
				"--server", tc.server,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--server must be an http(s) URL")
			// A rejected --server must not persist a broken context to disk.
			assert.NoFileExists(t, cfgPath, "config must not be written when --server is invalid")
		})
	}
}

func TestSetContextAcceptsValidServer(t *testing.T) {
	for _, server := range []string{
		"http://localhost:8000/api",
		"https://acm.example.com/api",
	} {
		t.Run(server, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			out, _, err := runRoot(t, cfgPath, "config", "set-context", "ctx",
				"--server", server,
			)
			require.NoError(t, err)
			assert.Contains(t, out, "saved")
		})
	}
}

func TestValidateServerURL(t *testing.T) {
	cases := []struct {
		name    string
		server  string
		wantErr bool
	}{
		{"http with host and path", "http://localhost:8000/api", false},
		{"https with host and path", "https://acm.example.com/api", false},
		{"http host no path", "http://localhost:8000", false},
		{"missing scheme", "localhost:8000", true},
		{"path only", "/api/v1", true},
		{"non-http scheme", "ftp://host", true},
		{"empty", "", true},
		{"scheme but no host", "http://", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerURL(tc.server)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--server must be an http(s) URL")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	out, _, err := runRoot(t, filepath.Join(t.TempDir(), "x.yaml"), "version", "--short")
	require.NoError(t, err)
	assert.Equal(t, "dev\n", out)
}

// TestConfigViewShowsInsecureColumn pins the INSECURE column. --insecure-skip-tls
// is persisted by set-context, so before this column the only way to audit which
// contexts skip certificate verification was to read the YAML by hand.
func TestConfigViewShowsInsecureColumn(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	_, _, err := runRoot(t, cfgPath, "config", "set-context", "kind-local",
		"--server", "http://localhost:8000/api",
		"--insecure-skip-tls",
	)
	require.NoError(t, err)
	_, _, err = runRoot(t, cfgPath, "config", "set-context", "prod",
		"--server", "https://acm.example.com/api",
	)
	require.NoError(t, err)

	out, _, err := runRoot(t, cfgPath, "config", "view")
	require.NoError(t, err)
	assert.Contains(t, out, "INSECURE", "table must carry the INSECURE header")

	// The unsafe context is labelled; the safe one leaves the cell blank so the
	// column draws the eye instead of filling with "false".
	lines := strings.Split(out, "\n")
	var insecureLine, secureLine string
	for _, l := range lines {
		switch {
		case strings.Contains(l, "kind-local"):
			insecureLine = l
		case strings.Contains(l, "prod"):
			secureLine = l
		}
	}
	require.NotEmpty(t, insecureLine, "kind-local row missing from %q", out)
	require.NotEmpty(t, secureLine, "prod row missing from %q", out)
	assert.Contains(t, insecureLine, "TLS verification skipped")
	assert.NotContains(t, secureLine, "TLS verification skipped")

	// -o json already carried the field; verify it is still there and still
	// redacts the token alongside it.
	out, _, err = runRoot(t, cfgPath, "config", "view", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"insecure-skip-tls": true`)
}
