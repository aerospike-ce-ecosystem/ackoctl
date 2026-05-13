package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatestTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Location", "https://example.test/aerospike-ce-ecosystem/ackoctl/releases/tag/v0.4.2")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c := &Client{
		HTTP: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BaseURL: srv.URL,
	}
	tag, err := c.LatestTag(context.Background())
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if tag != "v0.4.2" {
		t.Errorf("tag = %q, want v0.4.2", tag)
	}
}

func TestLatestTagBadLocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.test/no-tag-here/")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c := &Client{
		HTTP: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BaseURL: srv.URL,
	}
	if _, err := c.LatestTag(context.Background()); err == nil {
		t.Fatal("expected error on missing tag, got nil")
	}
}

func TestAssetURL(t *testing.T) {
	c := New()
	got := c.AssetURL("v0.1.0", "linux", "amd64")
	want := "https://github.com/aerospike-ce-ecosystem/ackoctl/releases/download/v0.1.0/ackoctl_0.1.0_linux_amd64.tar.gz"
	if got != want {
		t.Errorf("AssetURL = %q, want %q", got, want)
	}
}

func TestAssetName(t *testing.T) {
	got := AssetName("v1.2.3", "darwin", "arm64")
	want := "ackoctl_1.2.3_darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("AssetName = %q, want %q", got, want)
	}
}
