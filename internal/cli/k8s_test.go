package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
