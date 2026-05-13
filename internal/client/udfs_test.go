package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUDFsHappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"filename":"agg.lua","type":"LUA","hash":"deadbeef"},
			{"filename":"helpers.lua","type":"LUA","hash":"cafebabe"},
			{"filename":"stream.lua","type":"LUA","hash":"abad1dea"}
		]`))
	})

	mods, err := c.ListUDFs(context.Background(), "conn-1")
	require.NoError(t, err)
	require.Len(t, mods, 3)
	assert.Equal(t, "agg.lua", mods[0].Filename)
	assert.Equal(t, "LUA", mods[0].Type)
	assert.Equal(t, "deadbeef", mods[0].Hash)
	assert.Equal(t, "stream.lua", mods[2].Filename)
}

func TestUploadUDFSendsExactJSONBody(t *testing.T) {
	var seenBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"filename":"x.lua","type":"LUA","hash":"hh"}`))
	})

	const source = "function hi() return 1 end"
	out, err := c.UploadUDF(context.Background(), "conn-1", "x.lua", source)
	require.NoError(t, err)
	// Body MUST be exactly {"filename":..., "content":<lua source>} — no
	// multipart, no extra fields. Use JSONEq so key order is irrelevant.
	assert.JSONEq(t,
		`{"filename":"x.lua","content":"function hi() return 1 end"}`,
		string(seenBody),
	)
	// Confirm the JSON-encoded content round-trips back to the original Lua
	// source (no double encoding, no escaping surprises).
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(seenBody, &decoded))
	assert.Equal(t, source, decoded["content"])
	assert.Equal(t, "x.lua", out.Filename)
	assert.Equal(t, "LUA", out.Type)
	assert.Equal(t, "hh", out.Hash)
}

func TestUploadUDFFilenamePatternViolationSurfacesAs422(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","filename"],"msg":"string does not match regex","type":"value_error.str.regex"}]}`))
	})

	_, err := c.UploadUDF(context.Background(), "conn-1", "bad name!.lua", "x=1")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	// Validation lists arrive as JSON arrays in `detail` — confirm they made
	// it through the error parser intact rather than being elided as nil.
	assert.Contains(t, apiErr.Detail, "filename")
}

func TestUploadUDFRejectsEmptyArgs(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when arguments are empty")
	})
	_, err := c.UploadUDF(context.Background(), "conn-1", "", "x=1")
	require.Error(t, err)
	_, err = c.UploadUDF(context.Background(), "conn-1", "x.lua", "")
	require.Error(t, err)
}

func TestRemoveUDFSendsFilenameQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		assert.Equal(t, "agg.lua", r.URL.Query().Get("filename"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.RemoveUDF(context.Background(), "conn-1", "agg.lua"))
}

func TestRemoveUDFNotFoundSurfacesAs404(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"UDF module 'missing.lua' not found"}`))
	})
	err := c.RemoveUDF(context.Background(), "conn-1", "missing.lua")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "missing.lua")
}

func TestRemoveUDFRejectsEmptyFilename(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when filename is empty")
	})
	require.Error(t, c.RemoveUDF(context.Background(), "conn-1", ""))
}

func TestUDFConnIDIsPathEscaped(t *testing.T) {
	// connection IDs are opaque strings — defend against the rare admin who
	// uses spaces or slashes by validating url.PathEscape ran.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/udfs/conn with space", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	_, err := c.ListUDFs(context.Background(), "conn with space")
	require.NoError(t, err)
}
