package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaleK8sClusterRoundTrip(t *testing.T) {
	var seenBody map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/k8s/clusters/ns/c1/scale", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seenBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Scaling","size":5}`))
	})

	out, err := c.ScaleK8sCluster(context.Background(), "ns", "c1", 5)
	require.NoError(t, err)
	// The wire field is "size" per the FastAPI ScaleK8sClusterRequest model.
	assert.EqualValues(t, 5, seenBody["size"])
	assert.Equal(t, "Scaling", out["phase"])
	assert.EqualValues(t, 5, out["size"])
}

func TestScaleK8sClusterSurfacesServerValidationError(t *testing.T) {
	// Even if the client guard is bypassed, the FastAPI server enforces ge=1, le=8
	// and returns a 422 detail. The client should surface this as APIError.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"size must be between 1 and 8"}`))
	})

	_, err := c.ScaleK8sCluster(context.Background(), "ns", "c1", 99)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "size")
}

func TestListK8sClusterEventsBuildsQueryAndDecodes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/k8s/clusters/ns/c1/events", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "100", q.Get("limit"))
		assert.Equal(t, "Scaling", q.Get("category"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"type":"Normal","reason":"Scaled","message":"size 3 -> 5","count":1,"firstTimestamp":"2026-05-13T10:00:00Z","lastTimestamp":"2026-05-13T10:00:00Z","source":"acko","category":"Scaling"},
			{"type":"Warning","reason":"Drift","message":"config drift","count":2,"lastTimestamp":"2026-05-13T11:00:00Z","category":"Scaling"}
		]`))
	})

	events, err := c.ListK8sClusterEvents(context.Background(), "ns", "c1", 100, "Scaling")
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "Normal", events[0].Type)
	assert.Equal(t, "Scaled", events[0].Reason)
	assert.Equal(t, "size 3 -> 5", events[0].Message)
	assert.Equal(t, 1, events[0].Count)
	assert.Equal(t, "Scaling", events[0].Category)
	assert.Equal(t, "acko", events[0].Source)
	assert.Equal(t, "2026-05-13T11:00:00Z", events[1].LastTimestamp)
}

func TestListK8sClusterEventsOmitsCategoryWhenEmpty(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "50", q.Get("limit"))
		// category must be absent, not empty string, so the server applies its default.
		_, hasCategory := q["category"]
		assert.False(t, hasCategory, "category should be absent when not specified")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	events, err := c.ListK8sClusterEvents(context.Background(), "ns", "c1", 50, "")
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestListK8sClusterEventsSurfacesAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"AerospikeCluster ns/c1 not found"}`))
	})
	_, err := c.ListK8sClusterEvents(context.Background(), "ns", "c1", 50, "")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}
