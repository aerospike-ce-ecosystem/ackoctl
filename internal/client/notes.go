package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// UpsertSetNote creates or updates a set-level note. “note“ is the free
// text body; the server rejects empty / whitespace-only values. Use
// DeleteSetNote to remove a note explicitly — the previous "PUT empty
// string ⇒ delete" shortcut was a footgun and has been removed server-side.
func (c *BaseClient) UpsertSetNote(ctx context.Context, connID, namespace, set, note string) (*SetNote, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if namespace == "" || set == "" {
		return nil, fmt.Errorf("namespace and set are required for set note upsert")
	}
	path := "/notes/sets/" + url.PathEscape(connID) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(set)
	var out SetNote
	if err := c.Do(ctx, http.MethodPut, path, UpsertSetNoteRequest{Note: note}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSetNote removes the note for (conn, namespace, set). The endpoint
// is idempotent — deleting a non-existent note returns 204.
func (c *BaseClient) DeleteSetNote(ctx context.Context, connID, namespace, set string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if namespace == "" || set == "" {
		return fmt.Errorf("namespace and set are required for set note delete")
	}
	path := "/notes/sets/" + url.PathEscape(connID) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(set)
	return c.Do(ctx, http.MethodDelete, path, nil, nil, nil)
}

// ListSetNotes returns every set note for the connection, optionally
// filtered by namespace. An empty “namespace“ returns notes across all
// namespaces visible to the caller.
func (c *BaseClient) ListSetNotes(ctx context.Context, connID, namespace string) ([]SetNote, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	q := url.Values{}
	if namespace != "" {
		q.Set("namespace", namespace)
	}
	var out SetNotesListResponse
	if err := c.Do(ctx, http.MethodGet, "/notes/sets/"+url.PathEscape(connID), nil, q, &out); err != nil {
		return nil, err
	}
	return out.Notes, nil
}

// UpsertRecordNote creates or updates a record-level note. “pkType“ is
// optional — when empty, the server defaults to “auto“ which collapses to
// the heuristic-resolved persistence type (“string|int|bytes“). Pass an
// explicit “pkType“ for digit-only string keys to avoid INTEGER
// mis-classification.
func (c *BaseClient) UpsertRecordNote(ctx context.Context, connID, namespace, set, pk, pkType, note string) (*RecordNote, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if namespace == "" || set == "" || pk == "" {
		return nil, fmt.Errorf("namespace, set, and pk are required for record note upsert")
	}
	path := "/notes/records/" + url.PathEscape(connID) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(set) + "/" + url.PathEscape(pk)
	body := UpsertRecordNoteRequest{Note: note, PKType: pkType}
	var out RecordNote
	if err := c.Do(ctx, http.MethodPut, path, body, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRecordNote removes the record note. “pkType“ defaults to “auto“
// when empty — matching server behaviour. The endpoint is idempotent.
func (c *BaseClient) DeleteRecordNote(ctx context.Context, connID, namespace, set, pk, pkType string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if namespace == "" || set == "" || pk == "" {
		return fmt.Errorf("namespace, set, and pk are required for record note delete")
	}
	q := url.Values{}
	if pkType != "" {
		q.Set("pk_type", pkType)
	}
	path := "/notes/records/" + url.PathEscape(connID) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(set) + "/" + url.PathEscape(pk)
	return c.Do(ctx, http.MethodDelete, path, nil, q, nil)
}

// ListRecordNotes returns every record note for (conn, namespace, set).
// Both namespace and set are required — this is the recovery path for
// notes that the random-50 data browser scan does not surface.
func (c *BaseClient) ListRecordNotes(ctx context.Context, connID, namespace, set string) ([]RecordNote, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if namespace == "" || set == "" {
		return nil, fmt.Errorf("namespace and set are required for record notes list")
	}
	q := url.Values{}
	q.Set("ns", namespace)
	q.Set("set", set)
	var out RecordNotesListResponse
	if err := c.Do(ctx, http.MethodGet, "/notes/records/"+url.PathEscape(connID), nil, q, &out); err != nil {
		return nil, err
	}
	return out.Notes, nil
}
