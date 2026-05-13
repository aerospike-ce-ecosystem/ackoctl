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

func TestTruncateSetFullWipeOmitsBeforeLut(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sets/conn-1/test/users/truncate", r.URL.Path)
		// `null` and "absent" both mean full wipe at the wire level; we use
		// `omitempty` on a *int64 so the key is dropped entirely when
		// beforeLut is nil. Both the absent-key and explicit-null shapes are
		// accepted by the server, but we lock the actual emitted bytes here
		// so a future contract change is intentional.
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasKey := body["beforeLut"]
		assert.False(t, hasKey, "expected beforeLut to be omitted when nil; got %#v", body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"truncated test.users"}`))
	})

	require.NoError(t, c.TruncateSet(context.Background(), "conn-1", "test", "users", nil))
}

func TestTruncateSetSendsBeforeLutWhenProvided(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sets/conn-1/test/users/truncate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// JSON numbers decode to float64 in an `any` slot; compare via
		// EqualValues to avoid the float-vs-int type mismatch.
		assert.EqualValues(t, int64(1700000000000000000), body["beforeLut"])
		w.WriteHeader(http.StatusNoContent)
	})

	cutoff := int64(1700000000000000000)
	require.NoError(t, c.TruncateSet(context.Background(), "conn-1", "test", "users", &cutoff))
}

func TestTruncateSetAccepts204NoContent(t *testing.T) {
	// Some deployments return 204; the client must treat the empty body as
	// success rather than complaining about an empty JSON object.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.TruncateSet(context.Background(), "conn-1", "test", "users", nil))
}

func TestTruncateSetSurfacesServerError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"before_lut=0 is ambiguous; omit for a full truncate"}`))
	})
	cutoff := int64(0)
	err := c.TruncateSet(context.Background(), "conn-1", "test", "users", &cutoff)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "ambiguous")
}

func TestTruncateSetRejectsEmptyNamespaceOrSet(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when namespace/set is empty")
	})
	require.Error(t, c.TruncateSet(context.Background(), "conn-1", "", "users", nil))
	require.Error(t, c.TruncateSet(context.Background(), "conn-1", "test", "", nil))
}
