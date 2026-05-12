package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *BaseClient) ListConnections(ctx context.Context, workspaceID string) ([]Connection, error) {
	q := url.Values{}
	if workspaceID != "" {
		q.Set("workspace_id", workspaceID)
	}
	var out []Connection
	if err := c.Do(ctx, http.MethodGet, "/connections", nil, q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BaseClient) GetConnection(ctx context.Context, id string) (*Connection, error) {
	var out Connection
	if err := c.Do(ctx, http.MethodGet, "/connections/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) CreateConnection(ctx context.Context, req CreateConnectionRequest) (*Connection, error) {
	if req.WorkspaceID == "" && c.Workspace != "" {
		req.WorkspaceID = c.Workspace
	}
	var out Connection
	if err := c.Do(ctx, http.MethodPost, "/connections", req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) UpdateConnection(ctx context.Context, id string, req UpdateConnectionRequest) (*Connection, error) {
	var out Connection
	if err := c.Do(ctx, http.MethodPut, "/connections/"+url.PathEscape(id), req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) DeleteConnection(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/connections/"+url.PathEscape(id), nil, nil, nil)
}

func (c *BaseClient) ConnectionHealth(ctx context.Context, id string) (*ConnectionStatus, error) {
	var out ConnectionStatus
	if err := c.Do(ctx, http.MethodGet, "/connections/"+url.PathEscape(id)+"/health", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
