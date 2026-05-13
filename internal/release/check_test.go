package release

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsOutdated(t *testing.T) {
	tests := []struct {
		name           string
		current, other string
		want           bool
	}{
		{"older patch", "v0.1.0", "v0.1.1", true},
		{"older minor", "v0.1.5", "v0.2.0", true},
		{"older major", "v0.9.0", "v1.0.0", true},
		{"equal", "v0.1.0", "v0.1.0", false},
		{"newer patch", "v0.1.2", "v0.1.1", false},
		{"no v prefix", "0.1.0", "v0.2.0", true},
		{"dev build", "dev", "v0.1.0", false},
		{"snapshot", "0.1.0-snapshot", "v0.1.0", false},
		{"unparseable", "abc", "v0.1.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOutdated(tt.current, tt.other); got != tt.want {
				t.Errorf("IsOutdated(%s, %s) = %v, want %v", tt.current, tt.other, got, tt.want)
			}
		})
	}
}

func TestIsDevBuild(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"", true},
		{"dev", true},
		{"0.1.0-snapshot", true},
		{"v0.1.0-dirty", true},
		{"v0.1.0", false},
		{"0.1.0", false},
		{"v0.1.0-rc1", false},
		{"garbage", true},
	}
	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			if got := IsDevBuild(tt.v); got != tt.want {
				t.Errorf("IsDevBuild(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".version-check.json")

	want := &VersionCheck{
		CheckedAt: time.Now().UTC().Truncate(time.Second),
		LatestTag: "v0.4.2",
	}
	if err := WriteCache(path, want); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}
	got, err := ReadCache(path)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}
	if got.LatestTag != want.LatestTag {
		t.Errorf("LatestTag = %q, want %q", got.LatestTag, want.LatestTag)
	}
	if !got.CheckedAt.Equal(want.CheckedAt) {
		t.Errorf("CheckedAt = %v, want %v", got.CheckedAt, want.CheckedAt)
	}
}

func TestCachedLatestExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".version-check.json")

	// Fresh entry.
	_ = WriteCache(path, &VersionCheck{CheckedAt: time.Now().UTC(), LatestTag: "v9.9.9"})
	if got := CachedLatest(path); got != "v9.9.9" {
		t.Errorf("fresh CachedLatest = %q, want v9.9.9", got)
	}

	// Stale entry.
	_ = WriteCache(path, &VersionCheck{CheckedAt: time.Now().Add(-2 * CacheTTL), LatestTag: "v1.0.0"})
	if got := CachedLatest(path); got != "" {
		t.Errorf("stale CachedLatest = %q, want empty", got)
	}

	// Missing file.
	if got := CachedLatest(filepath.Join(dir, "nope.json")); got != "" {
		t.Errorf("missing CachedLatest = %q, want empty", got)
	}
}

func TestReadCacheMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".version-check.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCache(path); err == nil {
		t.Fatal("expected error on malformed cache, got nil")
	}
	// CachedLatest must swallow the error.
	if got := CachedLatest(path); got != "" {
		t.Errorf("CachedLatest on malformed = %q, want empty", got)
	}
}
