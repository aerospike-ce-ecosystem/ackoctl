package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// TruncateSet wipes every record in “namespace.setName“ on the connection,
// optionally up to a last-update-time cutoff. Destructive — callers must
// gate this behind a confirmation flag.
//
// “beforeLut“ is the cutoff in nanoseconds since CITRUS epoch:
//   - “nil“  — truncate the entire set (no cutoff).
//   - non-nil — truncate only records whose last-update-time is below
//     the given value. Server rejects an explicit “0“ to avoid the
//     silent "lut=0 means truncate-all" footgun at the info-command
//     layer; pass “nil“ for a full wipe.
//
// The endpoint returns either “{"message": "..."}“ or “204 No Content“;
// either is treated as success and the body is discarded.
func (c *BaseClient) TruncateSet(ctx context.Context, connID, namespace, setName string, beforeLut *int64) error {
	if namespace == "" || setName == "" {
		return fmt.Errorf("namespace and set name are required for set truncate")
	}
	path := "/sets/" + url.PathEscape(connID) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(setName) + "/truncate"
	return c.Do(ctx, http.MethodPost, path, TruncateSetRequest{BeforeLut: beforeLut}, nil, nil)
}
