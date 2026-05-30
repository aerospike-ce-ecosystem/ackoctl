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

func TestListRecordsBuildsQueryParams(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/records/conn-1", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "test", q.Get("ns"))
		assert.Equal(t, "users", q.Get("set"))
		assert.Equal(t, "50", q.Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[{"key":{"namespace":"test","set":"users","pk":"alice"},"meta":{"generation":1,"ttl":0},"bins":{"age":30}}],"total":1,"page":1,"pageSize":50,"hasMore":false}`))
	})

	out, err := c.ListRecords(context.Background(), "conn-1", "test", "users", 50)
	require.NoError(t, err)
	require.Len(t, out.Records, 1)
	assert.Equal(t, "alice", out.Records[0].Key.PK)
	// Record bins land in a map[string]any, decoded with json.Decoder.UseNumber,
	// so an integer bin arrives as json.Number — keeping int64 values above 2^53
	// exact instead of routing them through float64.
	age, ok := out.Records[0].Bins["age"].(json.Number)
	require.True(t, ok, "expected json.Number, got %T", out.Records[0].Bins["age"])
	assert.Equal(t, "30", age.String())
}

func TestListRecordsRequiresNamespace(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when namespace is empty")
	})
	_, err := c.ListRecords(context.Background(), "conn-1", "", "", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

func TestGetRecordHitsDetailPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/records/conn-1/detail", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "test", q.Get("ns"))
		assert.Equal(t, "users", q.Get("set"))
		assert.Equal(t, "alice", q.Get("pk"))
		assert.Equal(t, "string", q.Get("pk_type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":{"namespace":"test","set":"users","pk":"alice"},"meta":{"generation":2,"ttl":3600},"bins":{"name":"Alice"}}`))
	})
	rec, err := c.GetRecord(context.Background(), "conn-1", "test", "users", "alice", "string")
	require.NoError(t, err)
	assert.Equal(t, 2, rec.Meta.Generation)
	assert.Equal(t, "Alice", rec.Bins["name"])
}

func TestGetRecordSurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Record not found"}`))
	})
	_, err := c.GetRecord(context.Background(), "conn-1", "test", "users", "ghost", "")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "Record not found")
}

func TestPutRecordRoundTrip(t *testing.T) {
	var seen RecordWriteRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":{"namespace":"test","set":"u","pk":"alice"},"meta":{"generation":1,"ttl":300},"bins":{"age":30}}`))
	})
	ttl := 300
	rec, err := c.PutRecord(context.Background(), "conn-1", RecordWriteRequest{
		Key:    RecordKey{Namespace: "test", Set: "u", PK: "alice"},
		Bins:   map[string]any{"age": 30},
		TTL:    &ttl,
		PKType: "string",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", seen.Key.PK)
	assert.Equal(t, 300, *seen.TTL)
	assert.Equal(t, "string", seen.PKType)
	assert.Equal(t, "alice", rec.Key.PK)
}

func TestDeleteRecordSendsQueryParams(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		q := r.URL.Query()
		assert.Equal(t, "test", q.Get("ns"))
		assert.Equal(t, "users", q.Get("set"))
		assert.Equal(t, "alice", q.Get("pk"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteRecord(context.Background(), "conn-1", "test", "users", "alice", ""))
}

func TestDeleteBinHitsPathSegmentsAndPKType(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		// Path segments are escaped; the test server decodes %2F-free segments
		// back to their literal form when populating URL.Path.
		assert.Equal(t, "/v1/records/conn-1/test/users/alice/bins/age", r.URL.Path)
		assert.Equal(t, "string", r.URL.Query().Get("pk_type"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteBin(context.Background(), "conn-1", "test", "users", "alice", "age", "string"))
}

func TestDeleteBinOmitsEmptyPKType(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.RawQuery, "pk_type must not be sent when empty so server applies its `auto` default")
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteBin(context.Background(), "conn-1", "test", "users", "alice", "age", ""))
}

func TestDeleteBinRequiresAllPathParts(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when required parts are missing")
	})
	require.Error(t, c.DeleteBin(context.Background(), "conn-1", "", "users", "alice", "age", ""))
	require.Error(t, c.DeleteBin(context.Background(), "conn-1", "test", "", "alice", "age", ""))
	require.Error(t, c.DeleteBin(context.Background(), "conn-1", "test", "users", "", "age", ""))
	require.Error(t, c.DeleteBin(context.Background(), "conn-1", "test", "users", "alice", "", ""))
}

func TestDeleteBinSurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Record not found"}`))
	})
	err := c.DeleteBin(context.Background(), "conn-1", "test", "users", "ghost", "age", "")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestFilterRecordsPostsBodyAndDecodes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/records/conn-1/filter", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		var req FilteredQueryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test", req.Namespace)
		assert.Equal(t, "prefix", req.PKMatchMode)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[],"total":0,"page":1,"pageSize":25,"hasMore":false,"executionTimeMs":3,"scannedRecords":100,"returnedRecords":0}`))
	})
	resp, err := c.FilterRecords(context.Background(), "conn-1", FilteredQueryRequest{
		Namespace:   "test",
		Set:         "users",
		PKPattern:   "ali",
		PKMatchMode: "prefix",
		PageSize:    25,
	})
	require.NoError(t, err)
	assert.Equal(t, 100, resp.ScannedRecords)
}

func TestRecordMethodsRequireConnID(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when connID is empty")
	})

	_, err := c.GetRecord(context.Background(), "", "test", "users", "1", "")
	require.Error(t, err, "empty connID must be rejected client-side")

	err = c.DeleteRecord(context.Background(), "", "test", "users", "1", "")
	require.Error(t, err, "empty connID must be rejected client-side")
}
