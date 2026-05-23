package client

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIErrorNonJSONIsSummarized(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		body := "<html><body><h1>502 Bad Gateway</h1>" + strings.Repeat("x", 1000) + "</body></html>"
		_, _ = w.Write([]byte(body))
	})
	err := c.Do(context.Background(), http.MethodGet, "/anything", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "non-JSON response")
	assert.NotContains(t, apiErr.Detail, strings.Repeat("x", 500), "body must be truncated")
}

func TestParseAPIErrorMalformedJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{not-json`))
	})
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Contains(t, apiErr.Detail, "malformed JSON")
	// The underlying json.Unmarshal error reason must survive so operators
	// can tell "truncated body" apart from "non-JSON content" without
	// re-fetching the request. encoding/json reports "invalid character"
	// for the leading `{n` we sent above.
	assert.Contains(t, apiErr.Detail, "invalid character",
		"json.Unmarshal reason must be preserved in Detail")
	// The truncated body snippet must still be present for debugging.
	assert.Contains(t, apiErr.Detail, "{not-json",
		"truncated body snippet must remain in Detail")
}

func TestParseAPIErrorListDetailMarshaled(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","name"],"msg":"required"}]}`))
	})
	err := c.Do(context.Background(), http.MethodPost, "/connections", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Contains(t, apiErr.Detail, "required")
}

func TestVerboseLoggerEmitsMethodAndStatus(t *testing.T) {
	var buf bytes.Buffer
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	c.VerboseLogger = &buf
	var out map[string]any
	require.NoError(t, c.Do(context.Background(), http.MethodGet, "/echo", nil, nil, &out))
	got := buf.String()
	assert.Contains(t, got, "GET")
	assert.Contains(t, got, "/v1/echo")
	assert.Contains(t, got, "200")
	assert.NotContains(t, got, "Bearer", "verbose logger must never print the Authorization header")
}

func TestDoRejectsInvalidBaseURL(t *testing.T) {
	c := &BaseClient{BaseURL: "not a url", HTTPClient: http.DefaultClient}
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base url")
}

func TestDoEmptyBodyWithExpectedOut(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})
	var out map[string]any
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil, &out)
	require.Error(t, err, "empty body with a typed out target must surface, not silently zero-value")
	assert.Contains(t, err.Error(), "empty body")
}

func TestTruncateDoesNotSplitMultibyteCodepoint(t *testing.T) {
	// Each Korean syllable is 3 UTF-8 bytes; a byte-index slice at n=4 would
	// cut the second rune mid-codepoint and emit a U+FFFD replacement char.
	s := strings.Repeat("가", 10) // 10 runes, 30 bytes
	got := truncate(s, 4)
	assert.Equal(t, strings.Repeat("가", 4)+"...", got)
	assert.NotContains(t, got, "�", "truncate must not produce a replacement char")
	assert.Equal(t, 4, len([]rune(strings.TrimSuffix(got, "..."))), "exactly 4 runes kept")
}

func TestTruncateShorterThanLimitIsUnchanged(t *testing.T) {
	s := "한글"
	assert.Equal(t, s, truncate(s, 10))
}
