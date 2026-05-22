package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateNoteAsciiUnderLimit(t *testing.T) {
	assert.Equal(t, "hello", truncateNote("hello", 60))
}

func TestTruncateNoteAsciiExceedsLimit(t *testing.T) {
	long := strings.Repeat("x", 80)
	out := truncateNote(long, 60)
	assert.True(t, strings.HasSuffix(out, "..."))
	// 60 runes + "..."
	assert.Len(t, out, 63)
}

func TestTruncateNoteRespectsRunesNotBytes(t *testing.T) {
	// Korean characters are 3 bytes in UTF-8; slicing on bytes would split
	// codepoints. We must operate on runes so the truncated output stays
	// valid UTF-8.
	korean := strings.Repeat("가", 80) // 80 runes, 240 bytes
	out := truncateNote(korean, 5)
	assert.Equal(t, strings.Repeat("가", 5)+"...", out)
	assert.True(t, len([]rune(out)) == 8) // 5 runes + 3 dots
}

func TestTruncateNoteZeroLimitReturnsInput(t *testing.T) {
	// limit <= 0 means "do not truncate" — caller opts out of cropping.
	assert.Equal(t, "hello", truncateNote("hello", 0))
}

// runNoteCmd is a thin harness that wires the note command against an
// httptest server and captures stdout/stderr. We pass --server / --token /
// --output through the persistent root flags so resolveContext does not try
// to read ~/.ackoctl/config.yaml.
func runNoteCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	// HOME isolation: point the config loader at a temp dir so any side-car
	// config does not leak into the test outcome.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(append([]string{"--output", "json"}, args...))
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestNoteSetUpdateCommandRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/notes/sets/conn-1/test/users", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "hello", body["note"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","note":"hello","createdAt":"t","updatedAt":"t"}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := runNoteCmd(t, srv.URL,
		"note", "set", "update", "conn-1",
		"--namespace", "test", "--set", "users", "--note", "hello",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"setName": "users"`)
}

func TestNoteSetUpdateRejectsMissingFlags(t *testing.T) {
	// Server should not be hit when required flags are missing.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runNoteCmd(t, srv.URL, "note", "set", "update", "conn-1", "--namespace", "test")
	require.Error(t, err)
	// cobra reports missing required flag(s)
	assert.Contains(t, err.Error(), "required")
}

func TestNoteSetListRendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/notes/sets/conn-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notes":[{"connectionId":"conn-1","namespace":"test","setName":"users","note":"hi","createdAt":"t","updatedAt":"t","updatedBy":"alice"}]}`))
	}))
	t.Cleanup(srv.Close)

	// Default output is table; we leave --output off here to exercise table render.
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"note", "set", "list", "conn-1"})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())
	out := stdout.String()
	assert.Contains(t, out, "NAMESPACE")
	assert.Contains(t, out, "users")
	assert.Contains(t, out, "alice")
}

func TestNoteRecordUpdateSendsPKType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/notes/records/conn-1/test/users/42", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "vip", body["note"])
		// Canonical Pydantic alias is `pk_type`; verify ackoctl sends that
		// instead of the field-name spelling.
		assert.Equal(t, "string", body["pk_type"])
		_, hasFieldName := body["pkType"]
		assert.False(t, hasFieldName, "must not send pkType alongside pk_type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectionId":"conn-1","namespace":"test","setName":"users","pkText":"42","pkType":"string","note":"vip","createdAt":"t","updatedAt":"t"}`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runNoteCmd(t, srv.URL,
		"note", "record", "update", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "42",
		"--pk-type", "string", "--note", "vip",
	)
	require.NoError(t, err)
	// Server responds with the resolved type as `pkType` (field name), which
	// is what ackoctl prints in JSON output.
	assert.Contains(t, stdout, `"pkType": "string"`)
}

func TestNoteRecordDeleteSendsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/notes/records/conn-1/test/users/alice", r.URL.Path)
		assert.Equal(t, "string", r.URL.Query().Get("pk_type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runNoteCmd(t, srv.URL,
		"note", "record", "delete", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice", "--pk-type", "string",
		"--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted record note")
}

func TestNoteRecordDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runNoteCmd(t, srv.URL,
		"note", "record", "delete", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestNoteSetDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runNoteCmd(t, srv.URL,
		"note", "set", "delete", "conn-1",
		"--namespace", "test", "--set", "users",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestNoteRecordListRequiresNamespaceAndSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runNoteCmd(t, srv.URL, "note", "record", "list", "conn-1", "--namespace", "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestNoteRecordUpdateRejectsUnknownPKType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when --pk-type is invalid")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runNoteCmd(t, srv.URL,
		"note", "record", "update", "conn-1",
		"--namespace", "test", "--set", "s", "--pk", "k",
		"--pk-type", "integer", "--note", "memo",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto|string|int|bytes")
}
