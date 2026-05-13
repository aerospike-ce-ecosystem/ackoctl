package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ExecuteInfo posts an asinfo passthrough request to cluster-manager. ``req``
// must specify at least one command; when ``req.Node`` is empty the server
// fans out across every reachable node. When ``req.ReadOnly`` is true the
// server enforces its asinfo verb whitelist and rejects unknown verbs with
// 400 (surfaced as ``*APIError``); pass ``false`` to bypass the whitelist
// (and accept any write-capable verb).
func (c *BaseClient) ExecuteInfo(ctx context.Context, connID string, req ExecuteInfoRequest) (*ExecuteInfoResponse, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required for info execute")
	}
	if len(req.Commands) == 0 {
		return nil, fmt.Errorf("at least one command is required for info execute")
	}
	path := "/clusters/" + url.PathEscape(connID) + "/info"
	var out ExecuteInfoResponse
	if err := c.Do(ctx, http.MethodPost, path, req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
