package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*BaseClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(config.Context{Server: srv.URL, Token: "test-token", WorkspaceID: "ws-1"})
	return c, srv
}

func TestDoSendsBearerAuthAndJSON(t *testing.T) {
	var seen *http.Request
	var seenBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		seenBody, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"echo":"ok"}`))
	})

	var out struct {
		Echo string `json:"echo"`
	}
	body := map[string]any{"hello": "world"}
	require.NoError(t, c.Do(context.Background(), http.MethodPost, "/sample", body, nil, &out))

	assert.Equal(t, "/v1/sample", seen.URL.Path)
	assert.Equal(t, "Bearer test-token", seen.Header.Get("Authorization"))
	assert.Equal(t, "application/json", seen.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"hello":"world"}`, string(seenBody))
	assert.Equal(t, "ok", out.Echo)
}

func TestDoParsesFastAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Connection 'abc' not found"}`))
	})
	err := c.Do(context.Background(), http.MethodGet, "/connections/abc", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "abc")
}

func TestDoFallsBackToRawBodyWhenNoDetail(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream exploded"))
	})
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Contains(t, apiErr.Body, "upstream exploded")
}

func TestListConnectionsPropagatesWorkspaceQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/connections", r.URL.Path)
		assert.Equal(t, "ws-1", r.URL.Query().Get("workspace_id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a","name":"A","hosts":["h"],"port":3000,"workspaceId":"ws-1"}]`))
	})
	conns, err := c.ListConnections(context.Background(), c.Workspace)
	require.NoError(t, err)
	require.Len(t, conns, 1)
	assert.Equal(t, "A", conns[0].Name)
}

func TestCreateConnectionFillsDefaultWorkspace(t *testing.T) {
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&bodyJSON)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"n","hosts":["h"],"port":3000,"workspaceId":"ws-1"}`))
	})
	_, err := c.CreateConnection(context.Background(), CreateConnectionRequest{Name: "n", Hosts: []string{"h"}, Port: 3000})
	require.NoError(t, err)
	assert.Equal(t, "ws-1", bodyJSON["workspaceId"], "client should inject context workspace if request omits it")
}

func TestCreateConnectionSerializesDescription(t *testing.T) {
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&bodyJSON)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new","name":"n","hosts":["h"],"port":3000,"description":"hello"}`))
	})
	conn, err := c.CreateConnection(context.Background(), CreateConnectionRequest{
		Name: "n", Hosts: []string{"h"}, Port: 3000, Description: "hello",
	})
	require.NoError(t, err)
	// Outgoing request uses the cluster-manager wire field name.
	assert.Equal(t, "hello", bodyJSON["description"])
	assert.NotContains(t, bodyJSON, "note", "must not send legacy `note` field")
	// Response decode populates Description from the same wire field.
	assert.Equal(t, "hello", conn.Description)
}

func TestUpdateConnectionSerializesDescription(t *testing.T) {
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&bodyJSON)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc","name":"n","hosts":["h"],"port":3000,"description":"updated"}`))
	})
	desc := "updated"
	conn, err := c.UpdateConnection(context.Background(), "abc", UpdateConnectionRequest{Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, "updated", bodyJSON["description"])
	assert.NotContains(t, bodyJSON, "note")
	assert.Equal(t, "updated", conn.Description)
}

func TestConnectionHealthDecodesStatus(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connected":true,"nodeCount":3,"namespaceCount":1,"build":"8.1.0","edition":"Community"}`))
	})
	st, err := c.ConnectionHealth(context.Background(), "abc")
	require.NoError(t, err)
	assert.True(t, st.Connected)
	assert.Equal(t, 3, st.NodeCount)
	assert.Equal(t, "8.1.0", st.Build)
}

func TestClusterInfoReturnsRawMap(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/clusters/conn-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nodes":[{"id":"BB9"}],"namespaces":["test"]}`))
	})
	info, err := c.ClusterInfo(context.Background(), "conn-1")
	require.NoError(t, err)
	assert.NotNil(t, info["nodes"])
	assert.Equal(t, []any{"test"}, info["namespaces"])
}

func TestConfigureNamespaceReturnsMessage(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/clusters/conn-1/namespaces", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"applied 2 changes"}`))
	})
	msg, err := c.ConfigureNamespace(context.Background(), "conn-1", ConfigureNamespaceRequest{"namespace": "test"})
	require.NoError(t, err)
	assert.Equal(t, "applied 2 changes", msg)
}

func TestK8sListAndReconcile(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/k8s/clusters":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"namespace":"ns","name":"c1","phase":"Completed","size":3}]}`))
		case "/v1/k8s/clusters/ns/c1/force-reconcile":
			assert.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"namespace":"ns","name":"c1","phase":"Reconciling"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	list, err := c.ListK8sClusters(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Completed", list[0]["phase"])

	out, err := c.ForceReconcileK8sCluster(context.Background(), "ns", "c1")
	require.NoError(t, err)
	assert.Equal(t, "Reconciling", out["phase"])
}

func readAll(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
