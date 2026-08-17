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

// runConfigureNamespace drives `cluster configure-namespace` against an
// httptest server and reports the decoded request body, stderr, and whether the
// server was reached at all — the last part matters because every guard here
// exists to stop a request before it lands on a live cluster.
func runConfigureNamespace(t *testing.T, args ...string) (body map[string]any, stderr string, called bool, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	errBuf := &bytes.Buffer{}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(errBuf)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(append([]string{"cluster", "configure-namespace", "conn-1", "--name", "test"}, args...))
	root.SetContext(context.Background())
	err = root.Execute()
	return body, errBuf.String(), called, err
}

// paramArgs expands "k=v" pairs into repeated --param flags.
func paramArgs(params ...string) []string {
	out := make([]string, 0, len(params)*2)
	for _, p := range params {
		out = append(out, "--param", p)
	}
	return out
}

// Regression: cluster-manager's CreateNamespaceRequest body uses the JSON key
// `name` for the namespace. An earlier build of this command sent `namespace`
// instead, which the server rejected with HTTP 422
// ({"loc":["body","name"],"msg":"Field required"}).
//
// Also pins the wire shape of the now-typed body: the server declares
// memorySize and replicationFactor as ints, so they must arrive as JSON
// numbers. The previous map[string]any carried whatever the shell handed over,
// i.e. strings.
func TestClusterConfigureNamespaceWireShape(t *testing.T) {
	args := append(paramArgs("memorySize=2147483648", "replicationFactor=3"), "--yes")
	body, _, called, err := runConfigureNamespace(t, args...)
	require.NoError(t, err)
	require.True(t, called)

	// The namespace identifier must arrive as `name`, not `namespace`.
	assert.Equal(t, "test", body["name"], "body must use the `name` key for the namespace")
	_, hasLegacy := body["namespace"]
	assert.False(t, hasLegacy, "must not send the legacy `namespace` key")

	// Numbers, not strings — and exactly the three fields the server reads.
	assert.Equal(t, float64(2147483648), body["memorySize"])
	assert.Equal(t, float64(3), body["replicationFactor"])
	assert.Len(t, body, 3, "body must carry exactly name/memorySize/replicationFactor, got %v", body)
}

// A complete body still needs --yes — the gate is keyed on impact, not on which
// knobs were supplied — but it must not warn, because nothing is left for the
// server to fill in.
func TestClusterConfigureNamespaceBothKnobsWarnNothing(t *testing.T) {
	args := append(paramArgs("memorySize=2147483648", "replicationFactor=3"), "--yes")
	_, stderr, called, err := runConfigureNamespace(t, args...)
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotContains(t, stderr, "WARNING")
}

// TestClusterConfigureNamespacePartialBody is the core of the fix: only the
// knobs the operator supplied are sent, so changing one knob does not require
// restating the other. Because a cluster-manager without the paired fix
// substitutes its own default for an omitted field and applies it to the live
// namespace, the partial form is gated on --yes and warns about exactly which
// field the server may overwrite.
func TestClusterConfigureNamespacePartialBody(t *testing.T) {
	tests := []struct {
		name        string
		params      []string
		confirm     []string
		wantErr     string   // non-empty => expect this substring and no HTTP call
		wantWarnOf  string   // stderr must name the omitted knob
		wantBody    []string // keys that must be present
		wantOmitted []string // keys that must be absent from the wire
	}{
		{
			name:    "memorySize alone is refused without --yes",
			params:  []string{"memorySize=2147483648"},
			wantErr: "confirmation required (--yes)",
		},
		{
			name:    "replicationFactor alone is refused without --yes",
			params:  []string{"replicationFactor=3"},
			wantErr: "confirmation required (--yes)",
		},
		{
			name:    "refusal names the omitted knob and its server default",
			params:  []string{"memorySize=2147483648"},
			wantErr: "replicationFactor was not supplied (server default: 2)",
		},
		{
			name:    "refusal names the omitted memorySize default",
			params:  []string{"replicationFactor=3"},
			wantErr: "memorySize was not supplied (server default: 1073741824 bytes)",
		},
		{
			name:    "refusal points at the paired server fix",
			params:  []string{"replicationFactor=3"},
			wantErr: "aerospike-cluster-manager/pull/478",
		},
		{
			name:    "refusal names the namespace at risk",
			params:  []string{"replicationFactor=3"},
			wantErr: `namespace "test"`,
		},
		{
			name:        "--yes sends only replicationFactor",
			params:      []string{"replicationFactor=3"},
			confirm:     []string{"--yes"},
			wantWarnOf:  "memorySize was not supplied",
			wantBody:    []string{"name", "replicationFactor"},
			wantOmitted: []string{"memorySize"},
		},
		{
			name:        "-y sends only memorySize",
			params:      []string{"memorySize=2147483648"},
			confirm:     []string{"-y"},
			wantWarnOf:  "replicationFactor was not supplied",
			wantBody:    []string{"name", "memorySize"},
			wantOmitted: []string{"replicationFactor"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append(paramArgs(tc.params...), tc.confirm...)
			body, stderr, called, err := runConfigureNamespace(t, args...)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.False(t, called, "gate must refuse before any HTTP call")
				return
			}
			require.NoError(t, err)
			require.True(t, called, "confirmed run must reach the server")
			assert.Contains(t, stderr, "WARNING")
			assert.Contains(t, stderr, tc.wantWarnOf)
			assert.Contains(t, stderr, "aerospike-cluster-manager/pull/478",
				"the warning must point at the fix that makes it unnecessary")
			for _, k := range tc.wantBody {
				assert.Contains(t, body, k)
			}
			for _, k := range tc.wantOmitted {
				assert.NotContains(t, body, k,
					"an unsupplied knob must be absent from the body, not sent as a zero")
			}
			assert.Len(t, body, len(tc.wantBody))
		})
	}
}

// TestBuildConfigureNamespaceRequestRejects covers every input the allowlist and
// the pre-existing guards must refuse. cluster-manager declares only
// name/memorySize/replicationFactor and sets no `extra=`, so Pydantic's default
// extra="ignore" drops any other key while the request still returns 200 — the
// operator would believe a setting was applied.
func TestBuildConfigureNamespaceRequestRejects(t *testing.T) {
	tests := []struct {
		name    string
		params  []string
		wantErr string
	}{
		{
			// The documented example before this fix. Both knobs the operator
			// asked for were dropped and the namespace was resized to the
			// server defaults.
			name:    "previously documented example is refused",
			params:  []string{"high-water-disk-pct=70", "stop-writes-pct=90"},
			wantErr: `--param "high-water-disk-pct" would be silently ignored`,
		},
		{
			name:    "unknown key names the passthrough alternative",
			params:  []string{"memorySize=2000000", "replicationFactor=2", "nsup-period=120"},
			wantErr: "ackoctl info CONN_ID --allow-write",
		},
		{
			name:    "unknown key error names both accepted keys",
			params:  []string{"nsup-period=120"},
			wantErr: "reads only memorySize and replicationFactor",
		},
		{
			name:    "snake_case spelling of a real knob is still unknown",
			params:  []string{"memory_size=2000000"},
			wantErr: `--param "memory_size" would be silently ignored`,
		},
		{
			// Pre-existing guard, kept: a body carrying only the namespace name
			// is a no-op the server cannot act on.
			name:    "no params at all",
			params:  nil,
			wantErr: "at least one --param",
		},
		{
			name:    "non-numeric memorySize",
			params:  []string{"memorySize=2GiB"},
			wantErr: "must be a byte count in decimal digits",
		},
		{
			name:    "memorySize below the server minimum",
			params:  []string{"memorySize=1024"},
			wantErr: "below the server minimum of 1000000 bytes",
		},
		{
			name:    "non-numeric replicationFactor",
			params:  []string{"replicationFactor=two"},
			wantErr: "must be an integer between 1 and 8",
		},
		{
			name:    "replicationFactor above the CE cap",
			params:  []string{"replicationFactor=9"},
			wantErr: "outside the server range 1-8",
		},
		{
			name:    "replicationFactor zero",
			params:  []string{"replicationFactor=0"},
			wantErr: "outside the server range 1-8",
		},
		{
			// Pre-existing guard, kept: a repeated key silently overwrote the
			// earlier value (--param x=1 --param x=2 quietly dropped x=1).
			name:    "duplicate memorySize",
			params:  []string{"memorySize=2000000", "memorySize=3000000"},
			wantErr: `--param "memorySize" specified more than once`,
		},
		{
			name:    "duplicate replicationFactor",
			params:  []string{"replicationFactor=2", "replicationFactor=3"},
			wantErr: `--param "replicationFactor" specified more than once`,
		},
		{
			// Pre-existing guard, kept.
			name:    "malformed pair with no equals",
			params:  []string{"memorySize"},
			wantErr: "expected key=value",
		},
		{
			// Pre-existing guard, kept: strings.Cut("=90", "=") yields an empty
			// key with ok=true.
			name:    "empty key",
			params:  []string{"=90"},
			wantErr: "key must not be empty",
		},
		{
			// Pre-existing guard, kept.
			name:    "name is reserved for --name",
			params:  []string{"name=other"},
			wantErr: "name=... is reserved",
		},
		{
			// --param is StringArrayVar, so a comma-bearing value arrives whole
			// and is reported as the unknown key it is. StringSliceVar would
			// have split it into fragments and produced a different error about
			// a value the user never typed.
			name:    "comma-bearing value is reported whole",
			params:  []string{"storage-engine=device,/dev/sda"},
			wantErr: `--param "storage-engine" would be silently ignored`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildConfigureNamespaceRequest("test", tc.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestBuildConfigureNamespaceRequestAccepts(t *testing.T) {
	tests := []struct {
		name       string
		params     []string
		wantMemory *int64
		wantRF     *int
	}{
		{
			name:       "both knobs",
			params:     []string{"memorySize=2147483648", "replicationFactor=3"},
			wantMemory: ptrInt64(2147483648),
			wantRF:     ptrInt(3),
		},
		{
			name:       "order does not matter",
			params:     []string{"replicationFactor=1", "memorySize=1000000"},
			wantMemory: ptrInt64(1000000),
			wantRF:     ptrInt(1),
		},
		{
			// The whole point of the fix: one knob, and the other left alone.
			name:   "replicationFactor alone leaves memorySize unset",
			params: []string{"replicationFactor=3"},
			wantRF: ptrInt(3),
		},
		{
			name:       "memorySize alone leaves replicationFactor unset",
			params:     []string{"memorySize=2147483648"},
			wantMemory: ptrInt64(2147483648),
		},
		{
			// A byte count above 2^31 must survive: MemorySize is int64 for
			// exactly this reason.
			name:       "memory size beyond 32 bits",
			params:     []string{"memorySize=17179869184"},
			wantMemory: ptrInt64(17179869184),
		},
		{
			name:       "server minimum and maximum are inclusive",
			params:     []string{"memorySize=1000000", "replicationFactor=8"},
			wantMemory: ptrInt64(1000000),
			wantRF:     ptrInt(8),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildConfigureNamespaceRequest("test", tc.params)
			require.NoError(t, err)
			assert.Equal(t, "test", got.Name)
			assert.Equal(t, tc.wantMemory, got.MemorySize)
			assert.Equal(t, tc.wantRF, got.ReplicationFactor)
		})
	}
}

// Every rejection must happen before the HTTP call — the whole point is that
// the live namespace is never touched.
func TestClusterConfigureNamespaceRejectsBeforeAnyRequest(t *testing.T) {
	for _, args := range [][]string{
		paramArgs("high-water-disk-pct=70", "stop-writes-pct=90"),
		paramArgs("memorySize=1024", "replicationFactor=2"),
		paramArgs("memorySize=2147483648"), // partial, no --yes
		nil,
	} {
		_, _, called, err := runConfigureNamespace(t, args...)
		require.Error(t, err, "args %v must be rejected", args)
		assert.False(t, called, "args %v must not reach the server", args)
	}
}

func ptrInt64(v int64) *int64 { return &v }
func ptrInt(v int) *int       { return &v }

// The --yes gate is unconditional: a complete body mutates the running namespace
// just as a partial one does, so it needs confirmation too. The second case is
// the reason it cannot key on omission — an explicit memorySize=1000000 shrinks a
// live namespace to 1 MB with nothing omitted at all, so an omission-keyed gate
// would wave it straight through.
func TestClusterConfigureNamespaceRequiresYes(t *testing.T) {
	for _, params := range [][]string{
		{"memorySize=2147483648", "replicationFactor=3"},
		{"memorySize=1000000", "replicationFactor=2"}, // explicit shrink to 1 MB
	} {
		_, _, called, err := runConfigureNamespace(t, paramArgs(params...)...)
		require.Error(t, err, "params %v must be refused without --yes", params)
		assert.Contains(t, err.Error(), "confirmation required (--yes)")
		// The refusal names the environment, so an operator pointed at the wrong
		// context sees it before anything is applied.
		assert.Contains(t, err.Error(), "127.0.0.1")
		assert.False(t, called, "gate must refuse before any HTTP call")
	}

	complete := paramArgs("memorySize=2147483648", "replicationFactor=3")
	for _, flag := range []string{"--yes", "-y"} {
		_, _, called, err := runConfigureNamespace(t, append(complete, flag)...)
		require.NoError(t, err, "%s must proceed", flag)
		assert.True(t, called, "%s must reach the server", flag)
	}
}
