package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListGuides returns every operational guide registered for the workspace.
// Guides are workspace-scoped Markdown policy documents — at most one
// data-plane and one control-plane guide per workspace.
func (c *BaseClient) ListGuides(ctx context.Context, workspaceID string) ([]Guide, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace id is required to list guides")
	}
	var out GuidesListResponse
	if err := c.Do(ctx, http.MethodGet, "/guides/"+url.PathEscape(workspaceID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Guides, nil
}

// GetGuide fetches one operational guide. guideType must be "data-plane" or
// "control-plane"; cluster-manager returns 404 (surfaced as APIError) when the
// guide has not been registered for the workspace yet.
func (c *BaseClient) GetGuide(ctx context.Context, workspaceID, guideType string) (*Guide, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace id is required to get a guide")
	}
	if guideType == "" {
		return nil, fmt.Errorf("guide type is required (data-plane or control-plane)")
	}
	path := "/guides/" + url.PathEscape(workspaceID) + "/" + url.PathEscape(guideType)
	var out Guide
	if err := c.Do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
