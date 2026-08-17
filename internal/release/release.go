// Package release talks to the ackoctl GitHub Releases surface. It is the
// single source of truth for "what is the latest tag" and "where does the
// archive for this OS/arch live", shared by `ackoctl upgrade` and the
// startup version-check.
package release

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// Repo identifies the upstream binary host. The `latest` redirect and
	// download URLs below are derived from this single constant so a fork
	// can override it in tests by swapping the package variable.
	Repo = "aerospike-ce-ecosystem/ackoctl"

	// AssetName is the goreleaser archive name template. Mirror
	// `.goreleaser.yaml` archives.name_template — version is supplied
	// without the leading `v`.
	assetTemplate = "ackoctl_%s_%s_%s.tar.gz"

	// userAgent identifies ackoctl in HTTP logs so GitHub side can
	// distinguish self-update traffic from generic curl.
	userAgent = "ackoctl-release-client"
)

// tagPattern is the only shape allowed to reach a release URL. Every tag we
// build a URL from is interpolated straight into the path, and Go transmits
// path segments — including `..` — verbatim, so an unvalidated tag can steer
// both AssetURL and ChecksumsURL at a different repository's release assets.
// Because checksums.txt is fetched from the same tag-derived location, the
// sha256 check would then verify the substituted archive against its own
// checksum file and pass. Anchored, digits-only version fields with an
// optional pre-release / build-metadata suffix leaves no room for a separator
// (`/`), a dot-dot segment, or percent-encoding.
var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$`)

// ValidateTag rejects a release tag that is not a plain semver tag with a
// leading `v`. Call it before interpolating a tag into any URL — parseSemver
// is not a substitute: it discards everything after the first `-`/`+` before
// parsing, so `v1.2.3-<anything>` satisfies it whatever the suffix contains.
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("release tag must not be empty")
	}
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid release tag %q: want a semver tag such as v0.1.0, v1.2.3-rc1, or v1.2.3+build.5", tag)
	}
	return nil
}

// HTTPClient is the minimum surface we need from net/http.Client. Defined
// as an interface so tests can plug in a recorder without spinning up a
// real server.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client resolves release metadata. Zero value is unusable — use New().
type Client struct {
	HTTP    HTTPClient
	BaseURL string // overridable for tests; defaults to https://github.com
}

// New returns a Client that uses a fresh http.Client whose redirect
// policy is overridden so we can read the `Location` header of the
// `/releases/latest` redirect ourselves.
func New() *Client {
	return &Client{
		HTTP: &http.Client{
			// Bound the latest-tag round-trip so a stalled connection cannot
			// hang `ackoctl upgrade` or the startup version-check forever.
			// Consistent with internal/client and install.go's download timeout.
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BaseURL: "https://github.com",
	}
}

// LatestTag follows the `/releases/latest` redirect and returns the tag
// (e.g. "v0.1.3"). It does NOT hit the GitHub REST API — that requires
// a token for any non-trivial QPS and is unnecessary when we just need
// the tag string.
func (c *Client) LatestTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", c.BaseURL, Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("resolve latest tag: unexpected status %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("resolve latest tag: empty Location header")
	}
	idx := strings.LastIndex(loc, "/")
	if idx < 0 || idx == len(loc)-1 {
		return "", fmt.Errorf("resolve latest tag: cannot parse tag from %q", loc)
	}
	tag := loc[idx+1:]
	// Defense in depth: the Location header is the other source of tags that
	// feed AssetURL / ChecksumsURL, so hold it to the same shape as a
	// user-supplied --version rather than the old leading-`v` check.
	if err := ValidateTag(tag); err != nil {
		return "", fmt.Errorf("resolve latest tag: %w", err)
	}
	return tag, nil
}

// AssetURL returns the download URL for the binary archive matching tag/OS/arch.
// tag is expected to include the leading "v"; goos/goarch follow runtime conventions.
func (c *Client) AssetURL(tag, goos, goarch string) string {
	version := strings.TrimPrefix(tag, "v")
	asset := fmt.Sprintf(assetTemplate, version, goos, goarch)
	return fmt.Sprintf("%s/%s/releases/download/%s/%s", c.BaseURL, Repo, tag, asset)
}

// ChecksumsURL returns the canonical checksums.txt URL for a tag.
func (c *Client) ChecksumsURL(tag string) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/checksums.txt", c.BaseURL, Repo, tag)
}

// AssetName mirrors AssetURL's final path segment — handy when verifying a
// downloaded archive against a checksums.txt line.
func AssetName(tag, goos, goarch string) string {
	version := strings.TrimPrefix(tag, "v")
	return fmt.Sprintf(assetTemplate, version, goos, goarch)
}
