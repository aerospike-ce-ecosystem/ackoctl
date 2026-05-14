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

	// VerboseLogger receives one-line method/URL/status traces when non-nil.
	// The Authorization header is never written to this writer.
	VerboseLogger io.Writer
}

type Option func(*BaseClient)

func WithHTTPClient(h *http.Client) Option { return func(c *BaseClient) { c.HTTPClient = h } }
func WithWorkspace(w string) Option        { return func(c *BaseClient) { c.Workspace = w } }
func WithToken(t string) Option            { return func(c *BaseClient) { c.Token = t } }
func WithVerboseLogger(w io.Writer) Option { return func(c *BaseClient) { c.VerboseLogger = w } }

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
// {"detail": "..."} on errors; if that field is absent or the body is not
// JSON the raw body is used (truncated for non-JSON content types so a 502
// HTML page does not flood the terminal).
type APIError struct {
	StatusCode  int
	Detail      string
	Body        string
	ContentType string
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

	if c.VerboseLogger != nil {
		fmt.Fprintf(c.VerboseLogger, "ackoctl: -> %s %s\n", method, u)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("call %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()

	if c.VerboseLogger != nil {
		fmt.Fprintf(c.VerboseLogger, "ackoctl: <- %s %s %d\n", method, u, resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	}
	if out == nil {
		return nil
	}
	if len(raw) == 0 {
		// An empty body when the caller asked for a typed result is almost
		// always a server contract violation; surface it instead of returning
		// a zero-value struct.
		return fmt.Errorf("cluster-manager %d: empty body where a JSON object was expected", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 200))
	}
	return nil
}

func (c *BaseClient) url(path string, query url.Values) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base url %q: %w", c.BaseURL, err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid base url %q: missing scheme or host", c.BaseURL)
	}
	joined := base.JoinPath(pathPrefix, path)
	if len(query) > 0 {
		joined.RawQuery = query.Encode()
	}
	return joined.String(), nil
}

// parseAPIError extracts a {"detail": ...} message from a FastAPI-style error
// response. For non-JSON responses (HTML proxy pages, plain text) it returns
// a short status summary so terminals are not flooded.
func parseAPIError(status int, contentType string, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode:  status,
		Body:        string(body),
		ContentType: contentType,
	}
	if !isJSONContentType(contentType) {
		apiErr.Detail = fmt.Sprintf("non-JSON response (%s); body truncated: %s",
			summarizeContentType(contentType), truncate(string(body), 200))
		return apiErr
	}
	var envelope struct {
		Detail any `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		apiErr.Detail = fmt.Sprintf("malformed JSON error body: %s", truncate(string(body), 200))
		return apiErr
	}
	switch v := envelope.Detail.(type) {
	case string:
		apiErr.Detail = v
	case nil:
		apiErr.Detail = "(no detail provided by server)"
	default:
		b, err := json.Marshal(v)
		if err != nil {
			apiErr.Detail = fmt.Sprintf("%+v", v)
			return apiErr
		}
		apiErr.Detail = string(b)
	}
	return apiErr
}

func isJSONContentType(ct string) bool {
	ct = strings.ToLower(ct)
	// content-type may include charset, e.g. "application/json; charset=utf-8".
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(ct)
	return ct == "application/json" || ct == "application/problem+json" || strings.HasSuffix(ct, "+json")
}

func summarizeContentType(ct string) string {
	if ct == "" {
		return "no content-type"
	}
	return ct
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
