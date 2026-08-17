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

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
)

// runConfigureNamespace drives `cluster configure-namespace` against an
// httptest server and reports the decoded request body plus whether the server
// was reached at all — the second half matters because every guard here exists
// to stop a request before it lands on a live cluster.
func runConfigureNamespace(t *testing.T, params ...string) (body map[string]any, called bool, err error) {
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
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	args := []string{"cluster", "configure-namespace", "conn-1", "--name", "test"}
	for _, p := range params {
		args = append(args, "--param", p)
	}
	root.SetArgs(args)
	root.SetContext(context.Background())
	return body, called, root.Execute()
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
	body, called, err := runConfigureNamespace(t,
		"memorySize=2147483648",
		"replicationFactor=3",
	)
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

// TestBuildConfigureNamespaceRequestRejects covers every input the guard must
// refuse. cluster-manager declares only name/memorySize/replicationFactor and
// sets no `extra=`, so Pydantic's default extra="ignore" drops any other key —
// and the handler then interpolates its own defaults for whatever is missing
// into a live `set-config`. Every case below would otherwise have produced a
// 200 and a "configured successfully" message.
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
			name:    "snake_case spelling of a real knob is still unknown",
			params:  []string{"memory_size=2000000", "replicationFactor=2"},
			wantErr: `--param "memory_size" would be silently ignored`,
		},
		{
			name:    "no params at all",
			params:  nil,
			wantErr: "are both required",
		},
		{
			name:    "memorySize alone leaves replicationFactor to the server default",
			params:  []string{"memorySize=2147483648"},
			wantErr: "are both required",
		},
		{
			name:    "replicationFactor alone leaves memorySize to the server default",
			params:  []string{"replicationFactor=3"},
			wantErr: "are both required",
		},
		{
			name:    "missing-both error quotes the server defaults",
			params:  []string{"memorySize=2147483648"},
			wantErr: "memorySize=1073741824, replicationFactor=2",
		},
		{
			name:    "non-numeric memorySize",
			params:  []string{"memorySize=2GiB", "replicationFactor=2"},
			wantErr: "must be a byte count in decimal digits",
		},
		{
			name:    "memorySize below the server minimum",
			params:  []string{"memorySize=1024", "replicationFactor=2"},
			wantErr: "below the server minimum of 1000000 bytes",
		},
		{
			name:    "non-numeric replicationFactor",
			params:  []string{"memorySize=2000000", "replicationFactor=two"},
			wantErr: "must be an integer between 1 and 8",
		},
		{
			name:    "replicationFactor above the CE cap",
			params:  []string{"memorySize=2000000", "replicationFactor=9"},
			wantErr: "outside the server range 1-8",
		},
		{
			name:    "replicationFactor zero",
			params:  []string{"memorySize=2000000", "replicationFactor=0"},
			wantErr: "outside the server range 1-8",
		},
		{
			// A repeated key silently overwrote the earlier value before this
			// guard (--param x=1 --param x=2 quietly dropped x=1).
			name:    "duplicate memorySize",
			params:  []string{"memorySize=2000000", "memorySize=3000000", "replicationFactor=2"},
			wantErr: `--param "memorySize" specified more than once`,
		},
		{
			name:    "duplicate replicationFactor",
			params:  []string{"memorySize=2000000", "replicationFactor=2", "replicationFactor=3"},
			wantErr: `--param "replicationFactor" specified more than once`,
		},
		{
			name:    "malformed pair with no equals",
			params:  []string{"memorySize"},
			wantErr: "expected key=value",
		},
		{
			// strings.Cut("=90", "=") yields an empty key with ok=true.
			name:    "empty key",
			params:  []string{"=90"},
			wantErr: "key must not be empty",
		},
		{
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
		name   string
		params []string
		want   client.ConfigureNamespaceRequest
	}{
		{
			name:   "both knobs",
			params: []string{"memorySize=2147483648", "replicationFactor=3"},
			want:   client.ConfigureNamespaceRequest{Name: "test", MemorySize: 2147483648, ReplicationFactor: 3},
		},
		{
			name:   "order does not matter",
			params: []string{"replicationFactor=1", "memorySize=1000000"},
			want:   client.ConfigureNamespaceRequest{Name: "test", MemorySize: 1000000, ReplicationFactor: 1},
		},
		{
			// A byte count above 2^31 must survive: MemorySize is int64 for
			// exactly this reason.
			name:   "memory size beyond 32 bits",
			params: []string{"memorySize=17179869184", "replicationFactor=8"},
			want:   client.ConfigureNamespaceRequest{Name: "test", MemorySize: 17179869184, ReplicationFactor: 8},
		},
		{
			name:   "server minimum and maximum are inclusive",
			params: []string{"memorySize=1000000", "replicationFactor=8"},
			want:   client.ConfigureNamespaceRequest{Name: "test", MemorySize: 1000000, ReplicationFactor: 8},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildConfigureNamespaceRequest("test", tc.params)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Every rejection must happen before the HTTP call — the whole point is that
// the live namespace is never touched.
func TestClusterConfigureNamespaceRejectsBeforeAnyRequest(t *testing.T) {
	for _, params := range [][]string{
		{"high-water-disk-pct=70", "stop-writes-pct=90"},
		{"memorySize=2147483648"},
		{"memorySize=1024", "replicationFactor=2"},
		nil,
	} {
		_, called, err := runConfigureNamespace(t, params...)
		require.Error(t, err, "params %v must be rejected", params)
		assert.False(t, called, "params %v must not reach the server", params)
	}
}
