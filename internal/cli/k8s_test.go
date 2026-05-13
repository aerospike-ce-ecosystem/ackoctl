package cli

import (
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
		{Reason: "old", LastTimestamp: "2026-05-13T10:00:00Z"},      // 2h ago, before cutoff
		{Reason: "fresh", LastTimestamp: "2026-05-13T11:50:00Z"},    // 10m ago
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
