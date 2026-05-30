package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *BaseClient) ListIndexes(ctx context.Context, connID string) ([]SecondaryIndex, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out []SecondaryIndex
	if err := c.Do(ctx, http.MethodGet, "/indexes/"+url.PathEscape(connID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BaseClient) CreateIndex(ctx context.Context, connID string, req CreateIndexRequest) (*SecondaryIndex, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out SecondaryIndex
	if err := c.Do(ctx, http.MethodPost, "/indexes/"+url.PathEscape(connID), req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) DeleteIndex(ctx context.Context, connID, namespace, name string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("namespace and name are required for index delete")
	}
	q := url.Values{}
	q.Set("ns", namespace)
	q.Set("name", name)
	return c.Do(ctx, http.MethodDelete, "/indexes/"+url.PathEscape(connID), nil, q, nil)
}
