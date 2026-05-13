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

func TestGetK8sPodLogsBuildsPathAndQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/k8s/clusters/ns/c1/pods/c1-rack1-0/logs", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "200", q.Get("tail"))
		assert.Equal(t, "aerospike", q.Get("container"))
		assert.Equal(t, "1800", q.Get("since_seconds"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pod":"c1-rack1-0","logs":"line1\nline2\n","tailLines":200,"sinceSeconds":1800}`))
	})

	out, err := c.GetK8sPodLogs(context.Background(), "ns", "c1", "c1-rack1-0", K8sLogsOptions{
		Container:    "aerospike",
		Tail:         200,
		SinceSeconds: 1800,
	})
	require.NoError(t, err)
	assert.Equal(t, "c1-rack1-0", out.Pod)
	assert.Equal(t, "line1\nline2\n", out.Logs)
	assert.Equal(t, 200, out.TailLines)
	require.NotNil(t, out.SinceSeconds)
	assert.Equal(t, 1800, *out.SinceSeconds)
}

func TestGetK8sPodLogsOmitsOptionalQueryParams(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.False(t, q.Has("tail"), "tail must not be sent when zero")
		assert.False(t, q.Has("container"), "container must not be sent when empty")
		assert.False(t, q.Has("since_seconds"), "since_seconds must not be sent when zero")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pod":"p","logs":"","tailLines":500}`))
	})

	out, err := c.GetK8sPodLogs(context.Background(), "ns", "c1", "p", K8sLogsOptions{})
	require.NoError(t, err)
	assert.Equal(t, "p", out.Pod)
	assert.Equal(t, 500, out.TailLines)
	assert.Nil(t, out.SinceSeconds, "server-omitted sinceSeconds should round-trip as nil")
}

func TestListK8sPodsRoundTrip(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/k8s/clusters/ns/c1/pods", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Mixed states: ready/Running, NotReady but Running, and a Pending
		// pod that has not yet reported nodeId/rackId/hashes. The omitted
		// fields exercise the optional pointer round-trip.
		_, _ = w.Write([]byte(`[
			{
				"name":"c1-rack1-0",
				"podIP":"10.0.0.1",
				"hostIP":"172.16.0.5",
				"isReady":true,
				"phase":"Running",
				"image":"aerospike/aerospike-server:8.1.0",
				"dynamicConfigStatus":"Synced",
				"nodeId":"BB9000000000001",
				"rackId":1,
				"configHash":"sha256:abc",
				"podSpecHash":"sha256:def",
				"accessEndpoints":["10.0.0.1:3000","aerospike.example.com:3000"],
				"servicePort":3000,
				"clusterPort":3002
			},
			{
				"name":"c1-rack1-1",
				"podIP":"10.0.0.2",
				"hostIP":"172.16.0.6",
				"isReady":false,
				"phase":"Running",
				"image":"aerospike/aerospike-server:8.1.0",
				"dynamicConfigStatus":"Pending",
				"nodeId":"BB9000000000002",
				"rackId":1,
				"configHash":"sha256:abc",
				"podSpecHash":"sha256:old",
				"accessEndpoints":["10.0.0.2:3000"],
				"servicePort":3000,
				"clusterPort":3002
			},
			{
				"name":"c1-rack2-0",
				"isReady":false,
				"phase":"Pending",
				"image":"aerospike/aerospike-server:8.1.0"
			}
		]`))
	})

	pods, err := c.ListK8sPods(context.Background(), "ns", "c1")
	require.NoError(t, err)
	require.Len(t, pods, 3)

	// Ready pod: all fields populated; pointer fields should be non-nil.
	assert.Equal(t, "c1-rack1-0", pods[0].Name)
	assert.True(t, pods[0].IsReady)
	assert.Equal(t, "Running", pods[0].Phase)
	assert.Equal(t, "10.0.0.1", pods[0].PodIP)
	assert.Equal(t, "172.16.0.5", pods[0].HostIP)
	assert.Equal(t, "BB9000000000001", pods[0].NodeID)
	require.NotNil(t, pods[0].RackID)
	assert.Equal(t, 1, *pods[0].RackID)
	assert.Equal(t, "sha256:abc", pods[0].ConfigHash)
	assert.Equal(t, "sha256:def", pods[0].PodSpecHash)
	assert.Equal(t, []string{"10.0.0.1:3000", "aerospike.example.com:3000"}, pods[0].AccessEndpoints)
	require.NotNil(t, pods[0].ServicePort)
	assert.Equal(t, 3000, *pods[0].ServicePort)
	require.NotNil(t, pods[0].ClusterPort)
	assert.Equal(t, 3002, *pods[0].ClusterPort)
	assert.Equal(t, "Synced", pods[0].DynamicConfigStatus)

	// Not-ready but still scheduled: configHash matches the cluster but
	// podSpecHash is stale -- typical mid-rolling-restart state.
	assert.Equal(t, "c1-rack1-1", pods[1].Name)
	assert.False(t, pods[1].IsReady)
	assert.Equal(t, "Pending", pods[1].DynamicConfigStatus)
	assert.Equal(t, "sha256:old", pods[1].PodSpecHash)

	// Pending pod: server omitted everything the kubelet had not reported
	// yet. The optional pointer fields must surface as nil so callers can
	// distinguish "not reported" from "zero".
	assert.Equal(t, "c1-rack2-0", pods[2].Name)
	assert.False(t, pods[2].IsReady)
	assert.Equal(t, "Pending", pods[2].Phase)
	assert.Empty(t, pods[2].PodIP)
	assert.Empty(t, pods[2].HostIP)
	assert.Empty(t, pods[2].NodeID)
	assert.Nil(t, pods[2].RackID)
	assert.Nil(t, pods[2].ServicePort)
	assert.Nil(t, pods[2].ClusterPort)
	assert.Nil(t, pods[2].AccessEndpoints)
}

func TestListK8sPodsSurfacesAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"AerospikeCluster ns/c1 not found"}`))
	})
	_, err := c.ListK8sPods(context.Background(), "ns", "c1")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestGetK8sPodLogsSurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Pod 'ghost' does not belong to cluster 'c1'"}`))
	})

	_, err := c.GetK8sPodLogs(context.Background(), "ns", "c1", "ghost", K8sLogsOptions{Tail: 100})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "ghost")
}
