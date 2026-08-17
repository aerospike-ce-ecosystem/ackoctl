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

// TestValidateTag pins the guard that keeps a release tag from reshaping the
// URL it is interpolated into. Both AssetURL and ChecksumsURL derive their path
// from the tag, so a tag carrying path syntax redirects the archive AND its
// checksums.txt to the same substituted location — the sha256 check then
// verifies the substituted archive against its own checksums and passes.
func TestValidateTag(t *testing.T) {
	valid := []string{
		"v0.1.0",
		"v1.2.3",
		"v10.20.30",
		"v0.0.0",
		"v1.2.3-rc1",
		"v1.2.3-nightly.20260817",
		"v1.2.3+build.5",
		"v1.2.3-rc.1.build-2",
	}
	for _, tag := range valid {
		t.Run("valid/"+tag, func(t *testing.T) {
			if err := ValidateTag(tag); err != nil {
				t.Fatalf("ValidateTag(%q) = %v, want nil", tag, err)
			}
		})
	}

	invalid := []string{
		"",                     // no tag at all
		"0.1.0",                // missing leading v — callers normalise first
		"v1.2",                 // not three fields
		"v1.2.3.4",             // too many fields
		"latest",               // not a version
		"v1.2.3/extra",         // separator: escapes the tag path segment
		"v../../other/repo/v1", // dot-dot: normalises toward another repository
		"v1.2.3/../../x",       // dot-dot after a valid-looking prefix
		"v1.2.3%2f..%2fx",      // percent-encoded separator
		"v1.2.3?x=1",           // query string appended to the path
		"v1.2.3#frag",          // fragment
		"v1.2.3 ",              // trailing space
		"v1.-2.3",              // sign in a version field
		"v1.2.3-",              // empty pre-release suffix
		"v1.2.3_rc1",           // underscore is not a semver separator
		"vv1.2.3",              // doubled prefix
	}
	for _, tag := range invalid {
		t.Run("invalid/"+tag, func(t *testing.T) {
			if err := ValidateTag(tag); err == nil {
				t.Fatalf("ValidateTag(%q) = nil, want an error", tag)
			}
		})
	}
}

// parseSemver is deliberately lenient — it drops everything after the first
// `-`/`+` before parsing — so it cannot stand in for ValidateTag. This test
// documents that difference so nobody "simplifies" ValidateTag into a
// parseSemver call.
func TestParseSemverIsNotATagValidator(t *testing.T) {
	const hostile = "v1.2.3-/../../other/repo/releases/download/v1"
	if _, ok := parseSemver(hostile); !ok {
		t.Fatalf("parseSemver(%q) rejected the input; this test no longer documents anything", hostile)
	}
	if err := ValidateTag(hostile); err == nil {
		t.Fatalf("ValidateTag(%q) = nil, want an error", hostile)
	}
}

// A tag that fails validation must never make it into a URL, so LatestTag
// applies the same guard to GitHub's Location header as to a --version value.
//
// Each tag below starts with `v`, so the old leading-`v` check let it straight
// through into AssetURL/ChecksumsURL — a bare "nightly" would prove nothing
// here because the old check already rejected that.
func TestLatestTagRejectsNonSemverLocation(t *testing.T) {
	for _, tag := range []string{"vlatest", "v1.2", "v1.2.3.4", "v1.2.3-/../../x"} {
		t.Run(tag, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://example.test/aerospike-ce-ecosystem/ackoctl/releases/tag/"+tag)
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
				t.Fatalf("LatestTag accepted %q from the Location header, want an error", tag)
			}
		})
	}
}
