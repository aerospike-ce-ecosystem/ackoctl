package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *BaseClient) ClusterInfo(ctx context.Context, connID string) (ClusterInfo, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	out := ClusterInfo{}
	if err := c.Do(ctx, http.MethodGet, "/clusters/"+url.PathEscape(connID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BaseClient) ConfigureNamespace(ctx context.Context, connID string, req ConfigureNamespaceRequest) (string, error) {
	if connID == "" {
		return "", fmt.Errorf("connID is required")
	}
	var out MessageResponse
	if err := c.Do(ctx, http.MethodPost, "/clusters/"+url.PathEscape(connID)+"/namespaces", req, nil, &out); err != nil {
		return "", err
	}
	return out.Message, nil
}
