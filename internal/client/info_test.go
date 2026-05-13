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

func TestExecuteInfoFanOutHappyPath(t *testing.T) {
	var seen ExecuteInfoRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/clusters/conn-1/info", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"command":"build","node":"BB9020011AC4202","output":"8.1.0.0","error":null},
			{"command":"build","node":"BB9020011AC4203","output":"8.1.0.0","error":null}
		]}`))
	})

	out, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: []string{"build"},
		ReadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, out.Results, 2)
	assert.Equal(t, []string{"build"}, seen.Commands)
	assert.True(t, seen.ReadOnly, "client must send readOnly=true on the wire")
	assert.Empty(t, seen.Node, "node must be omitted on fan-out")
	assert.Equal(t, "8.1.0.0", out.Results[0].Output)
	assert.Equal(t, "BB9020011AC4203", out.Results[1].Node)
}

func TestExecuteInfoSingleNode(t *testing.T) {
	var seenBody map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seenBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"command":"status","node":"BB9020011AC4202","output":"ok"}]}`))
	})

	out, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: []string{"status"},
		Node:     "BB9020011AC4202",
		ReadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, out.Results, 1)
	assert.Equal(t, "BB9020011AC4202", seenBody["node"])
	assert.Equal(t, "BB9020011AC4202", out.Results[0].Node)
}

func TestExecuteInfoOmitsEmptyNode(t *testing.T) {
	// When Node is unset the JSON body must omit the field entirely so the
	// server picks fan-out mode. Sending "" would be ambiguous.
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&bodyJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})

	_, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: []string{"build"},
		ReadOnly: true,
	})
	require.NoError(t, err)
	_, hasNode := bodyJSON["node"]
	assert.False(t, hasNode, "empty node must be omitted from the request body")
}

func TestExecuteInfoSendsReadOnlyFalseExplicitly(t *testing.T) {
	// readOnly is *not* tagged omitempty: a false value must hit the wire so
	// the server clearly sees "user opted into write-capable verbs". If the
	// field were omitted the server's whitelist default would still apply.
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&bodyJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})

	_, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: []string{"set-config:context=service;proto-fd-max=20000"},
		ReadOnly: false,
	})
	require.NoError(t, err)
	val, hasReadOnly := bodyJSON["readOnly"]
	require.True(t, hasReadOnly, "readOnly must be present even when false")
	assert.Equal(t, false, val)
}

func TestExecuteInfoSurfacesWhitelistRejection(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"command 'set-config:...' not in read-only whitelist"}`))
	})

	_, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: []string{"set-config:context=service;proto-fd-max=20000"},
		ReadOnly: true,
	})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "whitelist")
}

func TestExecuteInfoRequiresCommands(t *testing.T) {
	// Empty Commands must fail client-side without a server round-trip — the
	// server would reject it anyway, but we surface a clearer error.
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be hit when commands are empty")
	})
	_, err := c.ExecuteInfo(context.Background(), "conn-1", ExecuteInfoRequest{
		Commands: nil,
		ReadOnly: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}

func TestExecuteInfoRequiresConnID(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be hit when connID is empty")
	})
	_, err := c.ExecuteInfo(context.Background(), "", ExecuteInfoRequest{
		Commands: []string{"build"},
		ReadOnly: true,
	})
	require.Error(t, err)
}
