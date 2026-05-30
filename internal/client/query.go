package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *BaseClient) ExecuteQuery(ctx context.Context, connID string, req QueryRequest) (*QueryResponse, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out QueryResponse
	if err := c.Do(ctx, http.MethodPost, "/query/"+url.PathEscape(connID), req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
