package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *BaseClient) ClusterInfo(ctx context.Context, connID string) (ClusterInfo, error) {
	out := ClusterInfo{}
	if err := c.Do(ctx, http.MethodGet, "/clusters/"+url.PathEscape(connID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BaseClient) ConfigureNamespace(ctx context.Context, connID string, req ConfigureNamespaceRequest) (string, error) {
	var out MessageResponse
	if err := c.Do(ctx, http.MethodPost, "/clusters/"+url.PathEscape(connID)+"/namespaces", req, nil, &out); err != nil {
		return "", err
	}
	return out.Message, nil
}
