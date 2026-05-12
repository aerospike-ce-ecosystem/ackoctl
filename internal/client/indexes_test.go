package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListIndexesDecodesSecondaryIndexes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/indexes/conn-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"idx_age","namespace":"test","set":"u","bin":"age","type":"numeric","state":"ready"}]`))
	})
	idx, err := c.ListIndexes(context.Background(), "conn-1")
	require.NoError(t, err)
	require.Len(t, idx, 1)
	assert.Equal(t, "idx_age", idx[0].Name)
	assert.Equal(t, "ready", idx[0].State)
}

func TestCreateIndexPostsBody(t *testing.T) {
	var got CreateIndexRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/indexes/conn-1", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"idx_age","namespace":"test","set":"u","bin":"age","type":"numeric","state":"building"}`))
	})
	idx, err := c.CreateIndex(context.Background(), "conn-1", CreateIndexRequest{
		Namespace: "test", Set: "u", Bin: "age", Name: "idx_age", Type: "numeric",
	})
	require.NoError(t, err)
	assert.Equal(t, "idx_age", got.Name)
	assert.Equal(t, "numeric", got.Type)
	assert.Equal(t, "building", idx.State)
}

func TestDeleteIndexSendsQueryParams(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/indexes/conn-1", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "test", q.Get("ns"))
		assert.Equal(t, "idx_age", q.Get("name"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteIndex(context.Background(), "conn-1", "test", "idx_age"))
}

func TestDeleteIndexRequiresFields(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit")
	})
	err := c.DeleteIndex(context.Background(), "conn-1", "", "idx")
	require.Error(t, err)
}
