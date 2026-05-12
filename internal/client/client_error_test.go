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
		_, _ = w.Write([]byte(`{"detail":`))
	})
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Contains(t, apiErr.Detail, "malformed JSON")
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
