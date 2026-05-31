package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
)

func TestIntFieldFromFloat64(t *testing.T) {
	v, ok := intField(map[string]any{"size": float64(3)}, "size")
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestIntFieldFromInt(t *testing.T) {
	v, ok := intField(map[string]any{"size": 5}, "size")
	assert.True(t, ok)
	assert.Equal(t, 5, v)
}

func TestIntFieldMissingKey(t *testing.T) {
	_, ok := intField(map[string]any{}, "size")
	assert.False(t, ok)
}

func TestIntFieldNilValue(t *testing.T) {
	_, ok := intField(map[string]any{"size": nil}, "size")
	assert.False(t, ok)
}

func TestIntFieldUnexpectedType(t *testing.T) {
	_, ok := intField(map[string]any{"size": "five"}, "size")
	assert.False(t, ok, "string value should not be coerced to int")
}

// TestIntFieldFromJSONNumber covers the real-world decode path: the REST
// client decodes raw-map responses with json.Decoder.UseNumber, so a K8sCluster
// "size" field arrives as json.Number. Without this case the scale-down safety
// guard would lose the current node count and misbehave.
func TestIntFieldFromJSONNumber(t *testing.T) {
	v, ok := intField(map[string]any{"size": json.Number("7")}, "size")
	assert.True(t, ok)
	assert.Equal(t, 7, v)
}

func TestIntFieldFromNonIntegerJSONNumber(t *testing.T) {
	_, ok := intField(map[string]any{"size": json.Number("3.5")}, "size")
	assert.False(t, ok, "a non-integer json.Number must not coerce to int")
}

func TestFilterEventsSinceZeroReturnsAll(t *testing.T) {
	events := []client.K8sClusterEvent{
		{Reason: "A", LastTimestamp: "2020-01-01T00:00:00Z"},
		{Reason: "B", LastTimestamp: "2026-05-13T11:00:00Z"},
	}
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	got := filterEventsSince(events, 0, now)
	assert.Len(t, got, 2)
}

func TestFilterEventsSinceDropsOlderThanCutoff(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	events := []client.K8sClusterEvent{
		{Reason: "old", LastTimestamp: "2026-05-13T10:00:00Z"},         // 2h ago, before cutoff
		{Reason: "fresh", LastTimestamp: "2026-05-13T11:50:00Z"},       // 10m ago
		{Reason: "exactCutoff", LastTimestamp: "2026-05-13T11:00:00Z"}, // exactly cutoff — kept under >= semantics
	}
	got := filterEventsSince(events, 1*time.Hour, now)
	reasons := make([]string, 0, len(got))
	for _, e := range got {
		reasons = append(reasons, e.Reason)
	}
	// Boundary value (>= cutoff) is included so users do not lose the edge event.
	assert.Equal(t, []string{"fresh", "exactCutoff"}, reasons)
}

func TestFilterEventsSinceFallsBackToFirstTimestamp(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	events := []client.K8sClusterEvent{
		// events.k8s.io/v1 EventSeries: lastTimestamp can be empty but firstTimestamp is set.
		{Reason: "seriesFresh", FirstTimestamp: "2026-05-13T11:50:00Z"},
		{Reason: "seriesStale", FirstTimestamp: "2020-01-01T00:00:00Z"},
	}
	got := filterEventsSince(events, 1*time.Hour, now)
	reasons := make([]string, 0, len(got))
	for _, e := range got {
		reasons = append(reasons, e.Reason)
	}
	assert.Equal(t, []string{"seriesFresh"}, reasons)
}

func TestFilterEventsSinceKeepsEventsWithNoParseableTimestamp(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	events := []client.K8sClusterEvent{
		{Reason: "empty"}, // both timestamps unset
		{Reason: "garbage", LastTimestamp: "not-a-timestamp", FirstTimestamp: "also-not"},
		{Reason: "old", LastTimestamp: "2020-01-01T00:00:00Z"},
	}
	got := filterEventsSince(events, 1*time.Hour, now)
	reasons := make([]string, 0, len(got))
	for _, e := range got {
		reasons = append(reasons, e.Reason)
	}
	// Events with no parseable timestamp at all are retained; the parseable-and-old "old" is dropped.
	assert.Equal(t, []string{"empty", "garbage"}, reasons)
}

func TestFilterEventsSinceEmptyInput(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	got := filterEventsSince(nil, 5*time.Minute, now)
	assert.Empty(t, got)
}

func TestSinceFlagToSecondsZero(t *testing.T) {
	got, err := sinceFlagToSeconds(0)
	require.NoError(t, err)
	assert.Equal(t, 0, got, "zero duration means --since not set")
}

func TestSinceFlagToSecondsWholeSeconds(t *testing.T) {
	got, err := sinceFlagToSeconds(30 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, 30, got)
}

func TestSinceFlagToSecondsRoundsUp(t *testing.T) {
	got, err := sinceFlagToSeconds(1500 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 2, got, "1.5s rounds up to 2s so caller does not lose logs")

	got, err = sinceFlagToSeconds(500 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, got, "sub-1s requests round up to the server's minimum of 1s")
}

func TestSinceFlagToSecondsRejectsOverMax(t *testing.T) {
	_, err := sinceFlagToSeconds(25 * time.Hour)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 1s and 24h")
}

func TestRackIDFieldNil(t *testing.T) {
	// A pod that has not yet been assigned to a rack -- the wire field is
	// omitted and the pointer surfaces as nil. Render as empty (not "0"
	// or "<nil>") so the column doesn't look like "rack 0".
	assert.Equal(t, "", rackIDField(nil))
}

func TestRackIDFieldZero(t *testing.T) {
	zero := 0
	assert.Equal(t, "0", rackIDField(&zero), "rack id 0 must render distinctly from nil")
}

func TestRackIDFieldPositive(t *testing.T) {
	rid := 7
	assert.Equal(t, "7", rackIDField(&rid))
}

func TestSinceFlagToSecondsAt24h(t *testing.T) {
	got, err := sinceFlagToSeconds(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 86400, got, "boundary value should be accepted")
}

// When the scale GET response carries no resolvable "size" key, the scale-down
// guard must NOT be skipped — it must require --yes (treat "cannot confirm
// current size" as "must confirm"). Without --yes the scale POST must not fire.
func TestK8sClusterScaleRequiresYesWhenCurrentSizeUnresolvable(t *testing.T) {
	scalePosted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			// No "size" key — node count nested elsewhere; intField returns ok=false.
			_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Completed","spec":{"size":5}}`))
			return
		}
		scalePosted = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Scaling","size":2}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"k8s", "cluster", "scale", "ns/c1", "--size", "2"})
	root.SetContext(context.Background())

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine current cluster size")
	assert.False(t, scalePosted, "scale must not be POSTed when current size is unconfirmed and --yes is absent")
}

// When the scale GET response carries a top-level "size", the scale-down guard
// must read it and refuse a shrink without --yes. This exercises the real
// decode path: the REST client decodes the raw K8sCluster map with
// json.Decoder.UseNumber, so "size" reaches intField as a json.Number. The
// guard would silently degrade to "cannot determine current size" if intField
// did not handle that type.
func TestK8sClusterScaleRefusesScaleDownFromTopLevelSize(t *testing.T) {
	scalePosted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			// Top-level "size": 5 — current node count is resolvable.
			_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Completed","size":5}`))
			return
		}
		scalePosted = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Scaling","size":2}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"k8s", "cluster", "scale", "ns/c1", "--size", "2"})
	root.SetContext(context.Background())

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing scale-down 5 -> 2")
	assert.False(t, scalePosted, "scale-down must not be POSTed without --yes")
}

// With --yes supplied, an unresolvable current size no longer blocks the scale.
func TestK8sClusterScaleProceedsWhenUnresolvableButYesGiven(t *testing.T) {
	scalePosted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Completed","spec":{"size":5}}`))
			return
		}
		scalePosted = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Scaling","size":2}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"k8s", "cluster", "scale", "ns/c1", "--size", "2", "--yes"})
	root.SetContext(context.Background())

	require.NoError(t, root.Execute())
	assert.True(t, scalePosted, "--yes must allow the scale to proceed despite an unconfirmed current size")
}

func TestSplitNamespacedName(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		wantNS    string
		wantName  string
		wantError bool
	}{
		{"valid", "ns/c1", "ns", "c1", false},
		{"name with dashes", "default/my-cluster", "default", "my-cluster", false},
		{"missing slash", "c1", "", "", true},
		{"empty namespace", "/c1", "", "", true},
		{"empty name", "ns/", "", "", true},
		{"empty input", "", "", "", true},
		// strings.Cut would fold the trailing segment into name
		// ("ns/c1/extra" -> name="c1/extra"); that bogus name path-escapes to
		// "%2F" and surfaces as a confusing server 404. Reject it client-side.
		{"extra segment", "ns/c1/extra", "", "", true},
		{"trailing slash", "ns/c1/", "", "", true},
		{"double slash", "ns//c1", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, err := splitNamespacedName(tt.arg)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNS, ns)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

// TestK8sCommandRejectsExtraPathSegment is an end-to-end guard that the
// scale command surfaces a clear client-side error for a malformed
// NAMESPACE/NAME argument and never reaches the network.
func TestK8sCommandRejectsExtraPathSegment(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	var errBuf bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&errBuf)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"k8s", "cluster", "scale", "ns/c1/extra", "--size", "2", "--yes"})
	root.SetContext(context.Background())

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected NAMESPACE/NAME")
	assert.False(t, hit, "a malformed NAMESPACE/NAME must fail before any network call")
}

// The CE node-count cap (1..8) is enforced client-side before any network
// call, so an out-of-range --size must fail fast with a clear message and never
// reach the server. This guards the validation at newK8sClusterScaleCmd against
// a regression that would otherwise only surface as a server-side webhook
// rejection (or, worse, a silently-truncated scale).
func TestK8sClusterScaleRejectsOutOfRangeSize(t *testing.T) {
	for _, size := range []string{"0", "-1", "9", "100"} {
		t.Run("size="+size, func(t *testing.T) {
			serverHit := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				serverHit = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Completed","size":3}`))
			}))
			t.Cleanup(srv.Close)

			root := NewRootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			t.Setenv("HOME", t.TempDir())
			t.Setenv("ACKOCTL_SERVER", srv.URL)
			t.Setenv("ACKOCTL_TOKEN", "test-token")
			root.SetArgs([]string{"k8s", "cluster", "scale", "ns/c1", "--size", size})
			root.SetContext(context.Background())

			err := root.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must be between 1 and 8")
			assert.False(t, serverHit, "out-of-range --size must be rejected before any network call")
		})
	}
}
