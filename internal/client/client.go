package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

const (
	defaultTimeout = 30 * time.Second
	pathPrefix     = "/v1"
)

type BaseClient struct {
	BaseURL    string
	Token      string
	Workspace  string
	HTTPClient *http.Client
}

type Option func(*BaseClient)

func WithHTTPClient(h *http.Client) Option { return func(c *BaseClient) { c.HTTPClient = h } }
func WithWorkspace(w string) Option        { return func(c *BaseClient) { c.Workspace = w } }
func WithToken(t string) Option            { return func(c *BaseClient) { c.Token = t } }

func New(ctx config.Context, opts ...Option) *BaseClient {
	transport := &http.Transport{}
	if ctx.InsecureSkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	c := &BaseClient{
		BaseURL:    strings.TrimRight(ctx.Server, "/"),
		Token:      ctx.Token,
		Workspace:  ctx.WorkspaceID,
		HTTPClient: &http.Client{Timeout: defaultTimeout, Transport: transport},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError represents a non-2xx response from cluster-manager. FastAPI returns
// {"detail": "..."} on errors; if that field is absent the raw body is used.
type APIError struct {
	StatusCode int
	Detail     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("cluster-manager %d: %s", e.StatusCode, e.Detail)
	}
	return fmt.Sprintf("cluster-manager %d: %s", e.StatusCode, e.Body)
}

// Do performs a JSON request. body may be nil. out may be nil for endpoints
// that return no body or where the caller does not care.
func (c *BaseClient) Do(ctx context.Context, method, path string, body any, query url.Values, out any) error {
	u, err := c.url(path, query)
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("call %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, raw)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 200))
	}
	return nil
}

func (c *BaseClient) url(path string, query url.Values) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base, err := url.Parse(c.BaseURL + pathPrefix + path)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	}
	return base.String(), nil
}

func parseAPIError(status int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: status, Body: string(body)}
	var envelope struct {
		Detail any `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		switch v := envelope.Detail.(type) {
		case string:
			apiErr.Detail = v
		case nil:
			// keep raw body
		default:
			b, _ := json.Marshal(v)
			apiErr.Detail = string(b)
		}
	}
	return apiErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
