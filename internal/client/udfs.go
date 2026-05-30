package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListUDFs returns every registered UDF module on the cluster behind
// “connID“. The cluster-manager endpoint fans out an
// “udf-list“ info call to a random node and parses the response into
// “UDFModule“ records.
func (c *BaseClient) ListUDFs(ctx context.Context, connID string) ([]UDFModule, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out []UDFModule
	if err := c.Do(ctx, http.MethodGet, "/udfs/"+url.PathEscape(connID), nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadUDF registers a Lua UDF module against the cluster. “content“ is
// the raw source text; cluster-manager writes it to a temp file named after
// “filename“ (the basename aerospike-py uses to derive the registered
// module name) and calls “udf_put“ under the hood. “filename“ must match
// “^[a-zA-Z0-9_.-]{1,255}$“ — violations come back as a 422 APIError.
func (c *BaseClient) UploadUDF(ctx context.Context, connID, filename, content string) (*UDFModule, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if filename == "" {
		return nil, fmt.Errorf("filename is required for udf upload")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required for udf upload")
	}
	var out UDFModule
	body := UploadUDFRequest{Filename: filename, Content: content}
	if err := c.Do(ctx, http.MethodPost, "/udfs/"+url.PathEscape(connID), body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveUDF removes a registered UDF module by filename. Not-found is
// surfaced as a 404 APIError by the server's “AerospikeError“ mapping;
// the call is otherwise idempotent for callers that swallow that case.
func (c *BaseClient) RemoveUDF(ctx context.Context, connID, filename string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if filename == "" {
		return fmt.Errorf("filename is required for udf remove")
	}
	q := url.Values{}
	q.Set("filename", filename)
	return c.Do(ctx, http.MethodDelete, "/udfs/"+url.PathEscape(connID), nil, q, nil)
}
