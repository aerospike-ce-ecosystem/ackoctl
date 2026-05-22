package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runUdfCmd wires the udf command tree against an httptest server and
// returns the captured stdout/stderr. Output format is forced to json so
// table-render whitespace does not creep into assertions.
func runUdfCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(append([]string{"--output", "json"}, args...))
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestUdfUploadReadsFileFromDisk(t *testing.T) {
	const source = "function hi() return 1 end\n"
	tmp := t.TempDir()
	src := filepath.Join(tmp, "my_module.lua")
	require.NoError(t, os.WriteFile(src, []byte(source), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]string
		require.NoError(t, json.Unmarshal(body, &decoded))
		// Basename of --file becomes the registered filename when --filename
		// is omitted; content is the verbatim file body (preserving newline).
		assert.Equal(t, "my_module.lua", decoded["filename"])
		assert.Equal(t, source, decoded["content"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"filename":"my_module.lua","type":"LUA","hash":"abc"}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runUdfCmd(t, srv.URL, "udf", "upload", "conn-1", "--file", src)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"filename": "my_module.lua"`)
	assert.Contains(t, stdout, `"hash": "abc"`)
}

func TestUdfUploadFilenameOverridesBasename(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "local_name.lua")
	require.NoError(t, os.WriteFile(src, []byte("x = 1\n"), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]string
		require.NoError(t, json.Unmarshal(body, &decoded))
		// Explicit --filename takes precedence over filepath.Base(--file).
		assert.Equal(t, "registered_name.lua", decoded["filename"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"filename":"registered_name.lua","type":"LUA","hash":"h"}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL,
		"udf", "upload", "conn-1",
		"--file", src,
		"--filename", "registered_name.lua",
	)
	require.NoError(t, err)
}

func TestUdfUploadRejectsMissingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --file does not exist")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL,
		"udf", "upload", "conn-1",
		"--file", filepath.Join(t.TempDir(), "does_not_exist.lua"),
	)
	require.Error(t, err)
}

func TestUdfUploadRejectsEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "empty.lua")
	require.NoError(t, os.WriteFile(src, nil, 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when source file is empty")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL, "udf", "upload", "conn-1", "--file", src)
	require.Error(t, err)
}

func TestUdfUploadRequiresFileFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --file is missing")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL, "udf", "upload", "conn-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestUdfRemoveRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit without --yes")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL,
		"udf", "remove", "conn-1", "--filename", "agg.lua",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestUdfRemoveHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		assert.Equal(t, "agg.lua", r.URL.Query().Get("filename"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	_, stderr, err := runUdfCmd(t, srv.URL,
		"udf", "remove", "conn-1", "--filename", "agg.lua", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Removed UDF module agg.lua")
}

func TestUdfListJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/udfs/conn-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"filename":"agg.lua","type":"LUA","hash":"deadbeef"},
			{"filename":"stream.lua","type":"LUA","hash":"abad1dea"}
		]`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runUdfCmd(t, srv.URL, "udf", "list", "conn-1")
	require.NoError(t, err)
	// JSON output preserves the wire shape — both modules visible with their
	// hashes, so callers can pipe through jq.
	assert.Contains(t, stdout, `"filename": "agg.lua"`)
	assert.Contains(t, stdout, `"filename": "stream.lua"`)
	assert.Contains(t, stdout, `"hash": "deadbeef"`)
}

func TestUdfListTableOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"filename":"agg.lua","type":"LUA","hash":"deadbeef"}]`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"udf", "list", "conn-1"})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())

	out := stdout.String()
	assert.Contains(t, out, "FILENAME")
	assert.Contains(t, out, "agg.lua")
	assert.Contains(t, out, "deadbeef")
}

// Regression: the upload command renders a *UDFModule pointer through
// output.Print's table path. A prior fix asserted the wrong type (`UDFModule`
// instead of `*UDFModule`) which panicked at runtime — the existing upload
// tests only exercised JSON output, missing the panic.
func TestUdfUploadDefaultTableDoesNotPanic(t *testing.T) {
	const source = "function hi() return 1 end\n"
	tmp := t.TempDir()
	src := filepath.Join(tmp, "hi.lua")
	require.NoError(t, os.WriteFile(src, []byte(source), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"filename":"hi.lua","type":"LUA","hash":"abc123"}`))
	}))
	t.Cleanup(srv.Close)

	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	// Default output is table — this is the path that previously panicked.
	root.SetArgs([]string{"udf", "upload", "conn-1", "--file", src})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())

	out := stdout.String()
	assert.Contains(t, out, "FILENAME")
	assert.Contains(t, out, "hi.lua")
	assert.Contains(t, out, "abc123")
}

func TestUdfUploadRejectsOversizedFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "huge.lua")
	// One byte over the cap is enough to trip the guard.
	require.NoError(t, os.WriteFile(src, make([]byte, maxUDFSourceSize+1), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when source file exceeds the size cap")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL, "udf", "upload", "conn-1", "--file", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the")
}

func TestUdfUploadRejectsDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --file is a directory")
	}))
	t.Cleanup(srv.Close)

	_, _, err := runUdfCmd(t, srv.URL, "udf", "upload", "conn-1", "--file", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}
