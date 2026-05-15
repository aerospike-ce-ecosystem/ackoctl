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

func TestExecuteQueryWithPredicate(t *testing.T) {
	var got QueryRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/query/conn-1", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[{"key":{"namespace":"test","set":"u","pk":"alice"},"meta":{"generation":1,"ttl":0},"bins":{"age":30}}],"executionTimeMs":7,"scannedRecords":42,"returnedRecords":1}`))
	})
	resp, err := c.ExecuteQuery(context.Background(), "conn-1", QueryRequest{
		Namespace:  "test",
		Set:        "u",
		Predicate:  &QueryPredicate{Bin: "age", Operator: "between", Value: 18, Value2: 30},
		SelectBins: []string{"name", "age"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test", got.Namespace)
	assert.NotNil(t, got.Predicate)
	assert.Equal(t, "between", got.Predicate.Operator)
	assert.Equal(t, float64(18), got.Predicate.Value)
	assert.Equal(t, []string{"name", "age"}, got.SelectBins)
	assert.Equal(t, 1, resp.ReturnedRecords)
}

func TestExecuteQuerySurfacesValidationError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"Set required for primary-key lookup"}`))
	})
	_, err := c.ExecuteQuery(context.Background(), "conn-1", QueryRequest{Namespace: "test", PrimaryKey: "alice"})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "Set required")
}
