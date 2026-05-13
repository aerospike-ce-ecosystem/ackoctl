package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (c *BaseClient) ListRecords(ctx context.Context, connID, namespace, set string, pageSize int) (*RecordListResponse, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	q := url.Values{}
	q.Set("ns", namespace)
	if set != "" {
		q.Set("set", set)
	}
	if pageSize > 0 {
		q.Set("pageSize", strconv.Itoa(pageSize))
	}
	var out RecordListResponse
	if err := c.Do(ctx, http.MethodGet, "/records/"+url.PathEscape(connID), nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) GetRecord(ctx context.Context, connID, namespace, set, pk, pkType string) (*AerospikeRecord, error) {
	if namespace == "" || set == "" || pk == "" {
		return nil, fmt.Errorf("namespace, set, and pk are all required for record get")
	}
	q := url.Values{}
	q.Set("ns", namespace)
	q.Set("set", set)
	q.Set("pk", pk)
	if pkType != "" {
		q.Set("pk_type", pkType)
	}
	var out AerospikeRecord
	if err := c.Do(ctx, http.MethodGet, "/records/"+url.PathEscape(connID)+"/detail", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) PutRecord(ctx context.Context, connID string, req RecordWriteRequest) (*AerospikeRecord, error) {
	var out AerospikeRecord
	if err := c.Do(ctx, http.MethodPost, "/records/"+url.PathEscape(connID), req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *BaseClient) DeleteRecord(ctx context.Context, connID, namespace, set, pk, pkType string) error {
	if namespace == "" || set == "" || pk == "" {
		return fmt.Errorf("namespace, set, and pk are all required for record delete")
	}
	q := url.Values{}
	q.Set("ns", namespace)
	q.Set("set", set)
	q.Set("pk", pk)
	if pkType != "" {
		q.Set("pk_type", pkType)
	}
	return c.Do(ctx, http.MethodDelete, "/records/"+url.PathEscape(connID), nil, q, nil)
}

// DeleteBin removes a single bin from a record. The endpoint is idempotent on
// the bin name — deleting a missing bin still returns 204. Removing the last
// bin from a record causes the entire record to disappear server-side; this
// matches standard Aerospike semantics and the cluster-manager docstring.
// ``pkType`` defaults to ``auto`` on the server when empty.
func (c *BaseClient) DeleteBin(ctx context.Context, connID, namespace, set, pk, binName, pkType string) error {
	if namespace == "" || set == "" || pk == "" || binName == "" {
		return fmt.Errorf("namespace, set, pk, and bin are all required for record delete-bin")
	}
	q := url.Values{}
	if pkType != "" {
		q.Set("pk_type", pkType)
	}
	path := "/records/" + url.PathEscape(connID) +
		"/" + url.PathEscape(namespace) +
		"/" + url.PathEscape(set) +
		"/" + url.PathEscape(pk) +
		"/bins/" + url.PathEscape(binName)
	return c.Do(ctx, http.MethodDelete, path, nil, q, nil)
}

func (c *BaseClient) FilterRecords(ctx context.Context, connID string, req FilteredQueryRequest) (*FilteredQueryResponse, error) {
	var out FilteredQueryResponse
	if err := c.Do(ctx, http.MethodPost, "/records/"+url.PathEscape(connID)+"/filter", req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
