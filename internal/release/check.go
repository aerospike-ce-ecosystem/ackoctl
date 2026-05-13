package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CacheTTL is how long a successful latest-tag lookup stays fresh on disk.
// 24h keeps the GitHub round-trip out of the hot path for daily users while
// still surfacing a new release within a workday.
const CacheTTL = 24 * time.Hour

// VersionCheck is the JSON payload persisted to ~/.ackoctl/.version-check.json.
// Kept tiny so we don't fight a malformed file — just nuke and refetch.
type VersionCheck struct {
	CheckedAt time.Time `json:"checked_at"`
	LatestTag string    `json:"latest_tag"`
}

// DefaultCachePath returns ~/.ackoctl/.version-check.json. Mirrors the
// config package's $HOME logic so the version-check honors HOME overrides
// in tests and ephemeral envs.
func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user home dir: %w", err)
	}
	return filepath.Join(home, ".ackoctl", ".version-check.json"), nil
}

// ReadCache returns the cached check or os.ErrNotExist if absent. A malformed
// file is reported as a normal error so callers can decide whether to delete
// and refetch.
func ReadCache(path string) (*VersionCheck, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v := &VersionCheck{}
	if err := json.Unmarshal(data, v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return v, nil
}

// WriteCache atomically writes the check to path. Atomic so a SIGINT mid-write
// can't leave a half-flushed file that future reads would reject.
func WriteCache(path string, v *VersionCheck) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".version-check-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// IsOutdated reports whether `current` is older than `latest`. Both inputs
// may carry a leading `v`. Pre-release builds (`dev`, `0.0.0-snapshot`) and
// unparseable strings return false — we never nag users running a local
// build off `main`, only those who installed a real tag.
func IsOutdated(current, latest string) bool {
	c, ok := parseSemver(current)
	if !ok {
		return false
	}
	l, ok := parseSemver(latest)
	if !ok {
		return false
	}
	return c.lessThan(l)
}

type semver struct {
	major, minor, patch int
}

func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	// Drop pre-release / build metadata before parsing — we treat
	// "0.1.3-rc1" as 0.1.3 for comparison purposes, which is fine because
	// pre-release builds shouldn't be installed via `ackoctl upgrade` anyway.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	out := semver{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}, false
		}
		switch i {
		case 0:
			out.major = n
		case 1:
			out.minor = n
		case 2:
			out.patch = n
		}
	}
	return out, true
}

func (a semver) lessThan(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// CachedLatest returns the latest tag from cache if it is younger than
// CacheTTL, or "" if no fresh entry exists. Cache read errors are swallowed —
// the version-check is best-effort and must never fail a user command.
func CachedLatest(cachePath string) string {
	v, err := ReadCache(cachePath)
	if err != nil {
		return ""
	}
	if time.Since(v.CheckedAt) > CacheTTL {
		return ""
	}
	return v.LatestTag
}

// RefreshCache resolves the latest tag and writes it to cachePath. Returns
// the new tag on success. Designed to run in a background goroutine after
// the user's command has completed, so a short network hiccup never adds
// to perceived CLI latency.
func RefreshCache(ctx context.Context, c *Client, cachePath string) (string, error) {
	tag, err := c.LatestTag(ctx)
	if err != nil {
		return "", err
	}
	if err := WriteCache(cachePath, &VersionCheck{
		CheckedAt: time.Now().UTC(),
		LatestTag: tag,
	}); err != nil {
		return tag, fmt.Errorf("write version-check cache: %w", err)
	}
	return tag, nil
}

// IsDevBuild reports whether `version` is a goreleaser-injected real tag.
// Used to suppress the outdated warning for `make build` / `go install`
// users running off main, where the warning would always fire and be
// useless.
func IsDevBuild(version string) bool {
	if version == "" || version == "dev" {
		return true
	}
	if strings.Contains(version, "snapshot") || strings.Contains(version, "dirty") {
		return true
	}
	// Tags like `v0.1.0-rc1` are real releases — keep them in scope. We
	// only bail out for clearly non-tagged builds.
	_, ok := parseSemver(version)
	return !ok
}
