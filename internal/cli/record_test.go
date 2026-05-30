package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRecordCmd mirrors runNoteCmd from note_test.go: drives the real cobra
// tree against an httptest server with HOME redirected so the config loader
// cannot leak global state into the assertion.
func runRecordCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
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

func TestRecordDeleteBinHitsServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/records/conn-1/test/users/alice/bins/age", r.URL.Path)
		assert.Equal(t, "string", r.URL.Query().Get("pk_type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice",
		"--bin", "age", "--pk-type", "string", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted bin")
	assert.Contains(t, stderr, "age")
}

func TestRecordDeleteBinRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice", "--bin", "age",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestRecordDeleteBinRequiresBin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call when --bin missing")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "delete-bin", "conn-1",
		"--namespace", "test", "--set", "users", "--pk", "alice", "--yes",
	)
	require.Error(t, err)
	// cobra reports missing required flag(s)
	assert.Contains(t, err.Error(), "required")
}

func TestRecordListRejectsOutOfRangePageSize(t *testing.T) {
	for _, pageSize := range []string{"0", "501", "-1"} {
		t.Run("pageSize="+pageSize, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --page-size is out of range")
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "list", "conn-1",
				"--namespace", "test", "--page-size", pageSize,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--page-size must be between 1 and 500")
		})
	}
}

func TestRecordQueryRejectsOutOfRangePagination(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"page=0", []string{"--page", "0"}, "--page must be 1 or greater"},
		{"pageSize=501", []string{"--page-size", "501"}, "--page-size must be between 1 and 500"},
		{"maxRecords=-1", []string{"--max-records", "-1"}, "--max-records must not be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when pagination flags are out of range")
			}))
			t.Cleanup(srv.Close)
			args := append([]string{"record", "query", "conn-1", "--namespace", "test"}, tc.args...)
			_, _, err := runRecordCmd(t, srv.URL, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRecordGetRejectsUnknownPKType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when --pk-type is invalid")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "get", "conn-1",
		"--namespace", "test", "--set", "s", "--pk", "k",
		"--pk-type", "integer",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto|string|int|bytes")
}

func TestRecordDeleteRejectsUnknownPKType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when --pk-type is invalid")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "delete", "conn-1",
		"--namespace", "test", "--set", "s", "--pk", "k",
		"--pk-type", "blob", "--yes",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto|string|int|bytes")
}

func TestRecordPutRejectsNonObjectBins(t *testing.T) {
	cases := []struct {
		name string
		bins string
		want string
	}{
		{"array", `[1,2,3]`, "array"},
		{"string", `"hello"`, "string"},
		{"number", `42`, "number"},
		{"null", `null`, "null"},
		{"empty", ``, "non-empty JSON object"},
		{"whitespace", `   `, "non-empty JSON object"},
		{"malformed", `not-json`, "JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --bins is not a JSON object")
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "put", "conn-1",
				"--namespace", "test", "--set", "s", "--pk", "k",
				"--bins", tc.bins,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRecordPutRejectsTTLBelowSentinel(t *testing.T) {
	// -1 (never expire) and 0 (namespace default) are valid sentinels; only
	// values below -1 are nonsensical and must be rejected before the request
	// is built. Mirrors #70's privilege guard for numeric flags.
	for _, ttl := range []string{"-2", "-100"} {
		t.Run("ttl="+ttl, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --ttl is below -1")
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "put", "conn-1",
				"--namespace", "test", "--set", "s", "--pk", "k",
				"--bins", `{"foo":1}`, "--ttl", ttl,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--ttl must be")
		})
	}
}

func TestRecordPutAcceptsValidTTL(t *testing.T) {
	// -1, 0, and a positive value must all reach the server.
	for _, ttl := range []string{"-1", "0", "3600"} {
		t.Run("ttl="+ttl, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"key":{"namespace":"test","set":"s","pk":"k"},"bins":{"foo":1}}`))
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "put", "conn-1",
				"--namespace", "test", "--set", "s", "--pk", "k",
				"--bins", `{"foo":1}`, "--ttl", ttl,
			)
			require.NoError(t, err)
			assert.True(t, called, "server should be called for valid --ttl=%s", ttl)
		})
	}
}

func TestRecordPutAcceptsJSONObjectBins(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":{"namespace":"test","set":"s","pk":"k"},"bins":{"foo":1}}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "put", "conn-1",
		"--namespace", "test", "--set", "s", "--pk", "k",
		"--bins", `{"foo":1}`,
	)
	require.NoError(t, err)
}

func TestRecordQueryRejectsUnknownPKMatchMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not be called when --pk-match-mode is invalid")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "query", "conn-1",
		"--namespace", "test", "--pk-pattern", "user-", "--pk-match-mode", "prefex",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exact|prefix|regex")
}

// queryFlagBadShapes is the shared rejection matrix for --filter and
// --predicate. "" and whitespace-only values must also be rejected when the
// flag is explicitly set, because the empty-string path in record.go is the
// "flag not supplied" branch — but the cobra parser still routes through the
// same flag, so the validator must fail before any HTTP request fires.
var queryFlagBadShapes = []struct {
	name string
	raw  string
	want string
}{
	{"null", `null`, "null"},
	{"array", `[]`, "array"},
	{"string", `"str"`, "string"},
	{"number", `42`, "number"},
	{"bool", `true`, "bool"},
	{"malformed", `{malformed`, "JSON object"},
	// Whitespace-only is non-empty at the cobra layer (so it bypasses the
	// "flag not set" shortcut in record.go) but trims to empty, which the
	// validator must catch with the non-empty-object message.
	{"whitespace", `   `, "non-empty JSON object"},
}

func TestRecordQueryRejectsNonObjectFilter(t *testing.T) {
	for _, tc := range queryFlagBadShapes {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --filter is not a JSON object")
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "query", "conn-1",
				"--namespace", "test", "--filter", tc.raw,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--filter")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRecordQueryRejectsNonObjectPredicate(t *testing.T) {
	for _, tc := range queryFlagBadShapes {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --predicate is not a JSON object")
			}))
			t.Cleanup(srv.Close)
			_, _, err := runRecordCmd(t, srv.URL,
				"record", "query", "conn-1",
				"--namespace", "test", "--predicate", tc.raw,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--predicate")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRecordQueryAcceptsJSONObjectFilter(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/records/conn-1/filter", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "query", "conn-1",
		"--namespace", "test", "--filter", `{"age":{"$gt":18}}`,
	)
	require.NoError(t, err)
	assert.True(t, called, "server should be called for a valid JSON-object --filter")
}

func TestRecordQueryAcceptsJSONObjectPredicate(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/records/conn-1/filter", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "query", "conn-1",
		"--namespace", "test", "--predicate", `{"bin":"age","op":"equals","value":42}`,
	)
	require.NoError(t, err)
	assert.True(t, called, "server should be called for a valid JSON-object --predicate")
}

// TestRecordQueryAcceptsEmptyFilter pins the "flag not set" path: an empty
// --filter must skip validation entirely and send no filter to the API.
func TestRecordQueryAcceptsEmptyFilter(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runRecordCmd(t, srv.URL,
		"record", "query", "conn-1",
		"--namespace", "test",
	)
	require.NoError(t, err)
	assert.True(t, called, "server should be called when --filter is omitted")
}
