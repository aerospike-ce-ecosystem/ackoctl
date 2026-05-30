package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertSetNoteBuildsPathAndBody(t *testing.T) {
	var seen UpsertSetNoteRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/notes/sets/conn-1/test/users", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","note":"hello","createdAt":"2026-05-13T00:00:00Z","updatedAt":"2026-05-13T00:00:00Z"}`))
	})

	out, err := c.UpsertSetNote(context.Background(), "conn-1", "test", "users", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", seen.Note)
	assert.Equal(t, "users", out.SetName)
	assert.Equal(t, "hello", out.Note)
}

func TestUpsertSetNoteRequiresPathParts(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when path parts are missing")
	})
	_, err := c.UpsertSetNote(context.Background(), "conn-1", "", "users", "hello")
	require.Error(t, err)
	_, err = c.UpsertSetNote(context.Background(), "conn-1", "test", "", "hello")
	require.Error(t, err)
}

func TestDeleteSetNoteHitsPathAndIgnoresEmpty204(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/notes/sets/conn-1/test/users", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteSetNote(context.Background(), "conn-1", "test", "users"))
}

func TestDeleteSetNoteSurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Connection 'conn-1' not found"}`))
	})
	err := c.DeleteSetNote(context.Background(), "conn-1", "test", "users")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestListSetNotesPassesNamespaceQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/notes/sets/conn-1", r.URL.Path)
		assert.Equal(t, "test", r.URL.Query().Get("namespace"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notes":[{"connectionId":"conn-1","namespace":"test","setName":"users","note":"hi","createdAt":"t","updatedAt":"t"}]}`))
	})
	notes, err := c.ListSetNotes(context.Background(), "conn-1", "test")
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "users", notes[0].SetName)
}

func TestListSetNotesOmitsEmptyNamespace(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.RawQuery, "namespace must not be sent when empty")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notes":[]}`))
	})
	notes, err := c.ListSetNotes(context.Background(), "conn-1", "")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestUpsertRecordNoteRoundTrip(t *testing.T) {
	var seen UpsertRecordNoteRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/notes/records/conn-1/test/users/alice", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","pkText":"alice","pkType":"string","note":"vip","createdAt":"t","updatedAt":"t"}`))
	})
	out, err := c.UpsertRecordNote(context.Background(), "conn-1", "test", "users", "alice", "string", "vip")
	require.NoError(t, err)
	assert.Equal(t, "vip", seen.Note)
	assert.Equal(t, "string", seen.PKType)
	assert.Equal(t, "alice", out.PKText)
	assert.Equal(t, "string", out.PKType)
}

func TestUpsertRecordNoteOmitsEmptyPKType(t *testing.T) {
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&bodyJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","pkText":"alice","pkType":"string","note":"x","createdAt":"t","updatedAt":"t"}`))
	})
	_, err := c.UpsertRecordNote(context.Background(), "conn-1", "test", "users", "alice", "", "x")
	require.NoError(t, err)
	// Wire key is the canonical Pydantic alias `pk_type`. Both spellings
	// must be absent so the server applies its `auto` default.
	_, hasAlias := bodyJSON["pk_type"]
	_, hasFieldName := bodyJSON["pkType"]
	assert.False(t, hasAlias, "empty pk_type must be omitted")
	assert.False(t, hasFieldName, "empty pkType (legacy spelling) must also be omitted")
}

func TestUpsertRecordNoteSendsCanonicalAlias(t *testing.T) {
	var bodyJSON map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&bodyJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","pkText":"42","pkType":"string","note":"x","createdAt":"t","updatedAt":"t"}`))
	})
	_, err := c.UpsertRecordNote(context.Background(), "conn-1", "test", "users", "42", "string", "x")
	require.NoError(t, err)
	// Send the alias, not the field name. populate_by_name=True accepts both
	// today but the alias is forward-compatible if Pydantic v3 enforces it.
	assert.Equal(t, "string", bodyJSON["pk_type"])
	_, hasFieldName := bodyJSON["pkType"]
	assert.False(t, hasFieldName, "must not send the field-name spelling alongside the alias")
}

func TestDeleteRecordNoteSendsPKType(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/notes/records/conn-1/test/users/alice", r.URL.Path)
		assert.Equal(t, "int", r.URL.Query().Get("pk_type"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteRecordNote(context.Background(), "conn-1", "test", "users", "alice", "int"))
}

func TestDeleteRecordNoteOmitsEmptyPKType(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteRecordNote(context.Background(), "conn-1", "test", "users", "alice", ""))
}

func TestListRecordNotesPassesNsAndSet(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/notes/records/conn-1", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "test", q.Get("ns"))
		assert.Equal(t, "users", q.Get("set"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notes":[{"connectionId":"conn-1","namespace":"test","setName":"users","pkText":"alice","pkType":"string","note":"vip","createdAt":"t","updatedAt":"t"}]}`))
	})
	notes, err := c.ListRecordNotes(context.Background(), "conn-1", "test", "users")
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "alice", notes[0].PKText)
}

func TestListRecordNotesRequiresNsAndSet(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when ns/set are missing")
	})
	_, err := c.ListRecordNotes(context.Background(), "conn-1", "", "users")
	require.Error(t, err)
	_, err = c.ListRecordNotes(context.Background(), "conn-1", "test", "")
	require.Error(t, err)
}

func TestNotesMethodsRequireConnID(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when connID is empty")
	})
	_, err := c.UpsertSetNote(context.Background(), "", "test", "users", "hello")
	require.Error(t, err)
	require.Error(t, c.DeleteSetNote(context.Background(), "", "test", "users"))
	_, err = c.ListSetNotes(context.Background(), "", "test")
	require.Error(t, err)
	_, err = c.UpsertRecordNote(context.Background(), "", "test", "users", "alice", "string", "vip")
	require.Error(t, err)
	require.Error(t, c.DeleteRecordNote(context.Background(), "", "test", "users", "alice", "int"))
	_, err = c.ListRecordNotes(context.Background(), "", "test", "users")
	require.Error(t, err)
}

func TestNotePathSegmentsAreEscaped(t *testing.T) {
	// Set names allow special characters in Aerospike (within limits); ensure
	// we escape rather than truncate at the slash.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// %20 round-trips through the test server as a literal space in URL.Path.
		assert.Equal(t, "/v1/notes/sets/conn-1/test/with space", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"with space","note":"x","createdAt":"t","updatedAt":"t"}`))
	})
	_, err := c.UpsertSetNote(context.Background(), "conn-1", "test", "with space", "x")
	require.NoError(t, err)
}
