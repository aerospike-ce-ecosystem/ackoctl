package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJSONScalarBareword(t *testing.T) {
	v, err := parseJSONScalar("alice")
	require.NoError(t, err)
	assert.Equal(t, "alice", v)
}

func TestParseJSONScalarNumber(t *testing.T) {
	v, err := parseJSONScalar("30")
	require.NoError(t, err)
	assert.Equal(t, float64(30), v)
}

func TestParseJSONScalarList(t *testing.T) {
	v, err := parseJSONScalar(`[1,2,3]`)
	require.NoError(t, err)
	assert.Equal(t, []any{float64(1), float64(2), float64(3)}, v)
}

func TestParseJSONScalarQuotedString(t *testing.T) {
	v, err := parseJSONScalar(`"alice"`)
	require.NoError(t, err)
	assert.Equal(t, "alice", v)
}

func TestParseJSONScalarRejectsTruncatedList(t *testing.T) {
	_, err := parseJSONScalar("[1,2")
	require.Error(t, err, "truncated JSON list must not fall back to a plain string predicate")
	assert.Contains(t, err.Error(), "looks like JSON")
}

func TestParseJSONScalarRejectsBrokenObject(t *testing.T) {
	_, err := parseJSONScalar(`{"bad`)
	require.Error(t, err)
}

func TestParseJSONScalarNullLiteral(t *testing.T) {
	v, err := parseJSONScalar("null")
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestParseJSONScalarBoolLiteral(t *testing.T) {
	v, err := parseJSONScalar("true")
	require.NoError(t, err)
	assert.Equal(t, true, v)
}

func TestParseJSONScalarNegativeNumber(t *testing.T) {
	v, err := parseJSONScalar("-3.14")
	require.NoError(t, err)
	assert.Equal(t, -3.14, v)
}

// ---------------------------------------------------------------------------
// query exec predicate validation
// ---------------------------------------------------------------------------

// runQueryCmd builds a root command, points it at srvURL via env, and forces
// JSON output. Predicate validation runs before any HTTP call, so the
// validation tests pass a server that fails the test if it is ever hit.
func runQueryCmd(t *testing.T, srvURL string, args ...string) (string, error) {
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
	return stdout.String(), err
}

func TestQueryExecRejectsOutOfRangeMaxRecords(t *testing.T) {
	for _, maxRecords := range []string{"-1", "1000001"} {
		t.Run("maxRecords="+maxRecords, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when --max-records is out of range")
			}))
			t.Cleanup(srv.Close)
			_, err := runQueryCmd(t, srv.URL,
				"query", "exec", "conn-1",
				"--namespace", "test", "--max-records", maxRecords,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--max-records must be 0")
		})
	}
}

func TestQueryExecPredicateRequiresValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when a predicate has no --value")
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1",
		"--namespace", "test", "--bin", "age", "--op", "equals",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--value is required when building a predicate")
}

func TestQueryExecBetweenRequiresValue2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --op between is missing the upper bound")
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1",
		"--namespace", "test", "--bin", "age", "--op", "between", "--value", "10",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--value2 is required when --op is 'between'")
}

func TestQueryExecBetweenRequiresValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --op between is missing the lower bound")
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1",
		"--namespace", "test", "--bin", "age", "--op", "between", "--value2", "20",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--value is required when building a predicate")
}

func TestQueryExecValue2RejectedWithoutBetween(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --value2 is paired with a non-between op")
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1",
		"--namespace", "test", "--bin", "age", "--op", "equals", "--value", "10", "--value2", "20",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--value2 is only valid when --op is 'between'")
}

func TestQueryExecValue2AloneRequiresBinAndOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when --value2 is supplied without --bin/--op")
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1", "--namespace", "test", "--value2", "20",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--bin and --op are required together")
}

func TestQueryExecBetweenRoundTrip(t *testing.T) {
	var pred map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/query/conn-1", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		pred, _ = body["predicate"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[],"executionTimeMs":1,"scannedRecords":0,"returnedRecords":0}`))
	}))
	t.Cleanup(srv.Close)
	_, err := runQueryCmd(t, srv.URL,
		"query", "exec", "conn-1",
		"--namespace", "test", "--bin", "age", "--op", "between", "--value", "10", "--value2", "20",
	)
	require.NoError(t, err)
	require.NotNil(t, pred)
	assert.Equal(t, "between", pred["operator"])
	assert.Equal(t, float64(10), pred["value"])
	assert.Equal(t, float64(20), pred["value2"])
}
