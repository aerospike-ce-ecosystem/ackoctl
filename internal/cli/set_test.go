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

// runSetCmd wires the set command against an httptest server so the test can
// observe the exact wire shape without depending on ~/.ackoctl/config.yaml.
func runSetCmd(t *testing.T, srvURL string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(args)
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestExtractSetsAcrossNamespaces(t *testing.T) {
	info := map[string]any{
		"namespaces": []any{
			map[string]any{
				"name": "test",
				"sets": []any{
					// memoryDataBytes is what cluster-manager 1.3.x emits today.
					map[string]any{"name": "users", "objects": float64(123), "memoryDataBytes": float64(4096)},
					map[string]any{"name": "orders", "object_count": float64(50)},
				},
			},
			map[string]any{
				"name": "analytics",
				"sets": []any{
					map[string]any{"name": "events", "data-used-bytes": float64(99)},
				},
			},
		},
	}

	all, drifted := extractSets(info, "", nil)
	assert.False(t, drifted)
	assert.Len(t, all, 3)
	assert.Equal(t, "test", all[0].Namespace)
	assert.Equal(t, "users", all[0].Name)
	assert.Equal(t, float64(123), all[0].Objects)
	assert.Equal(t, float64(4096), all[0].MemUsed)

	// memory key fallback via object_count alias
	assert.Equal(t, "orders", all[1].Name)
	assert.Equal(t, float64(50), all[1].Objects)
	assert.Nil(t, all[1].MemUsed)

	// data-used-bytes alias (legacy server)
	assert.Equal(t, "events", all[2].Name)
	assert.Equal(t, float64(99), all[2].MemUsed)
}

func TestCellStringNilBecomesEmpty(t *testing.T) {
	assert.Equal(t, "", cellString(nil))
	assert.Equal(t, "0", cellString(float64(0)))
	assert.Equal(t, "4096", cellString(float64(4096)))
}

func TestExtractSetsFilterByNamespace(t *testing.T) {
	info := map[string]any{
		"namespaces": []any{
			map[string]any{"name": "a", "sets": []any{map[string]any{"name": "s1"}}},
			map[string]any{"name": "b", "sets": []any{map[string]any{"name": "s2"}}},
		},
	}
	only, drifted := extractSets(info, "b", nil)
	assert.False(t, drifted)
	assert.Len(t, only, 1)
	assert.Equal(t, "b", only[0].Namespace)
	assert.Equal(t, "s2", only[0].Name)
}

func TestExtractSetsHandlesEmptyOrMissing(t *testing.T) {
	empty1, drifted := extractSets(map[string]any{}, "", nil)
	assert.False(t, drifted)
	assert.Empty(t, empty1)

	empty2, drifted := extractSets(map[string]any{"namespaces": []any{}}, "", nil)
	assert.False(t, drifted)
	assert.Empty(t, empty2)
}

func TestExtractSetsFlagsSchemaDrift(t *testing.T) {
	var buf bytes.Buffer
	rows, drifted := extractSets(map[string]any{
		"namespaces": []any{
			map[string]any{"name": "x", "sets": "not-a-list"},
		},
	}, "", &buf)
	assert.Empty(t, rows)
	assert.True(t, drifted, "expected drift when sets is not a list")
	assert.True(t, strings.Contains(buf.String(), "sets is not a list"), "expected warning, got %q", buf.String())
}

func TestExtractSetsDriftWithEmptyNamespacesList(t *testing.T) {
	// `namespaces: []` is a valid empty response, not drift.
	rows, drifted := extractSets(map[string]any{"namespaces": []any{}}, "", nil)
	assert.Empty(t, rows)
	assert.False(t, drifted)
}

func TestExtractSetsDriftWhenNamespacesNotAList(t *testing.T) {
	var buf bytes.Buffer
	rows, drifted := extractSets(map[string]any{"namespaces": "oops"}, "", &buf)
	assert.Empty(t, rows)
	assert.True(t, drifted)
	assert.True(t, strings.Contains(buf.String(), "namespaces` is not a list"), "expected warning, got %q", buf.String())
}

// --- set truncate -----------------------------------------------------------

func TestSetTruncateRequiresYes(t *testing.T) {
	// --yes is the safety gate; without it the server must never be reached.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runSetCmd(t, srv.URL,
		"set", "truncate", "conn-1",
		"--namespace", "test", "--set", "users",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestSetTruncateFullWipeOmitsBeforeLut(t *testing.T) {
	// Default invocation (no --before-lut) must NOT send the key. cluster-
	// manager rejects an explicit 0; we rely on cobra's Changed() check to
	// stay silent when the user didn't pass the flag.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sets/conn-1/test/users/truncate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasKey := body["beforeLut"]
		assert.False(t, hasKey, "beforeLut must be absent for full-set truncate; got %#v", body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runSetCmd(t, srv.URL,
		"set", "truncate", "conn-1",
		"--namespace", "test", "--set", "users", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "full set")
}

func TestSetTruncateParsesBeforeLut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// JSON decodes integers into float64 inside an `any` slot.
		assert.EqualValues(t, int64(1700000000000000000), body["beforeLut"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"truncated up to lut"}`))
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runSetCmd(t, srv.URL,
		"set", "truncate", "conn-1",
		"--namespace", "test", "--set", "users",
		"--before-lut", "1700000000000000000",
		"--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "lut=1700000000000000000")
}

func TestSetTruncateRequiresNamespaceAndSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected server call with missing required flags")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runSetCmd(t, srv.URL,
		"set", "truncate", "conn-1", "--namespace", "test", "--yes",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}
