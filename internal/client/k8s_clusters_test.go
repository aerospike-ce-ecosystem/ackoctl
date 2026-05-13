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
