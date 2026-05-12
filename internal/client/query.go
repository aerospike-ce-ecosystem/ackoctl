package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *BaseClient) ExecuteQuery(ctx context.Context, connID string, req QueryRequest) (*QueryResponse, error) {
	var out QueryResponse
	if err := c.Do(ctx, http.MethodPost, "/query/"+url.PathEscape(connID), req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
