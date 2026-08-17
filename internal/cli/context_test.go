package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

// writeTestContext persists ctx as the current context in a throwaway config
// file and returns its path for `--config`. It goes through config.Save rather
// than hand-rolling YAML so a schema change breaks the test loudly instead of
// silently loading an empty config.
func writeTestContext(t *testing.T, ctx config.Context) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, config.Save(path, &config.Config{
		APIVersion:     config.APIVersion,
		Kind:           config.Kind,
		CurrentContext: ctx.Name,
		Contexts:       []config.Context{ctx},
	}))
	return path
}

// TestInsecureSkipTLSWarnsWithoutVerbose pins the fix for a warning that only
// fired under --verbose. --insecure-skip-tls is persisted by `config
// set-context`, so the operator who enabled it once against kind saw nothing
// at all on later runs — including after the context was repointed at a real
// server, where the bearer token then travelled over an unverified connection.
func TestInsecureSkipTLSWarnsWithoutVerbose(t *testing.T) {
	const warning = "TLS verification is disabled"

	tests := []struct {
		name     string
		args     []string // extra flags, appended after `connection list`
		insecure bool     // context value written to the config file
		wantWarn bool
	}{
		{
			name:     "insecure context, no --verbose",
			insecure: true,
			wantWarn: true,
		},
		{
			name:     "insecure context, with --verbose",
			args:     []string{"--verbose"},
			insecure: true,
			wantWarn: true,
		},
		{
			name:     "insecure via flag only, no --verbose",
			args:     []string{"--insecure-skip-tls"},
			wantWarn: true,
		},
		{
			name:     "secure context stays quiet",
			wantWarn: false,
		},
		{
			name:     "secure context stays quiet under --verbose",
			args:     []string{"--verbose"},
			wantWarn: false,
		},
		{
			// --insecure-skip-tls=false must beat a context that has it set,
			// and must not warn: pickBool honours the explicit CLI false.
			name:     "explicit --insecure-skip-tls=false overrides context",
			args:     []string{"--insecure-skip-tls=false"},
			insecure: true,
			wantWarn: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(srv.Close)

			cfgPath := writeTestContext(t, config.Context{
				Name:            "kind-local",
				Server:          srv.URL,
				Token:           "test-token",
				InsecureSkipTLS: tc.insecure,
			})

			args := append([]string{"--config", cfgPath, "connection", "list"}, tc.args...)
			root := NewRootCmd()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			root.SetOut(stdout)
			root.SetErr(stderr)
			t.Setenv("HOME", t.TempDir())
			root.SetArgs(args)
			root.SetContext(context.Background())
			require.NoError(t, root.Execute())

			if tc.wantWarn {
				assert.Contains(t, stderr.String(), warning,
					"warning must reach stderr regardless of --verbose")
				// The resolved target is echoed so the operator can tell which
				// environment is unverified.
				assert.Contains(t, stderr.String(), "kind-local")
			} else {
				assert.NotContains(t, stderr.String(), warning)
			}
			// stdout carries only command output — a warning there would break
			// `-o json | jq` pipelines.
			assert.NotContains(t, stdout.String(), warning)
		})
	}
}

func TestDescribeTarget(t *testing.T) {
	tests := []struct {
		name string
		ctx  config.Context
		want string
	}{
		{
			name: "named context reports host only",
			ctx:  config.Context{Name: "prod", Server: "https://acm.example.com/api"},
			want: `context "prod" (acm.example.com)`,
		},
		{
			name: "server-only context has no name",
			ctx:  config.Context{Server: "http://localhost:8000/api"},
			want: "server localhost:8000",
		},
		{
			// An unparseable server must not panic or drop the identity — fall
			// back to the raw string so the message still says something.
			name: "unparseable server falls back to the raw value",
			ctx:  config.Context{Name: "broken", Server: "not a url"},
			want: `context "broken" (not a url)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, describeTarget(tc.ctx))
		})
	}
}
