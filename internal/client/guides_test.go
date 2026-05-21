package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGuidesBuildsPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/guides/ws-default", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"guides":[{"workspaceId":"ws-default","guideType":"data-plane","title":"DP","content":"# DP","createdAt":"t","updatedAt":"t"}]}`))
	})
	guides, err := c.ListGuides(context.Background(), "ws-default")
	require.NoError(t, err)
	require.Len(t, guides, 1)
	assert.Equal(t, "data-plane", guides[0].GuideType)
}

func TestListGuidesRequiresWorkspace(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit without a workspace")
	})
	_, err := c.ListGuides(context.Background(), "")
	require.Error(t, err)
}

func TestGetGuideBuildsPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/guides/ws-team-a/control-plane", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceId":"ws-team-a","guideType":"control-plane","title":"CP","content":"# CP\n\nbody","createdAt":"t","updatedAt":"t","updatedBy":"alice"}`))
	})
	g, err := c.GetGuide(context.Background(), "ws-team-a", "control-plane")
	require.NoError(t, err)
	assert.Equal(t, "control-plane", g.GuideType)
	assert.Equal(t, "alice", g.UpdatedBy)
	assert.Contains(t, g.Content, "# CP")
}

func TestGetGuideRequiresArgs(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when args are missing")
	})
	_, err := c.GetGuide(context.Background(), "", "data-plane")
	require.Error(t, err)
	_, err = c.GetGuide(context.Background(), "ws-default", "")
	require.Error(t, err)
}

func TestGetGuideSurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"No 'data-plane' guide is registered for this workspace"}`))
	})
	_, err := c.GetGuide(context.Background(), "ws-default", "data-plane")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestGuidePathSegmentsAreEscaped(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// A workspace id containing a space must be escaped, not truncated.
		assert.Equal(t, "/v1/guides/ws team/data-plane", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceId":"ws team","guideType":"data-plane","title":"DP","content":"x","createdAt":"t","updatedAt":"t"}`))
	})
	_, err := c.GetGuide(context.Background(), "ws team", "data-plane")
	require.NoError(t, err)
}
