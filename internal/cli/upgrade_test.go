package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUpgradeTagUnsetResolvesLatest(t *testing.T) {
	// An empty raw value means the user did not pass --version at all, so
	// the caller must fall back to GitHub's "latest" tag resolution.
	tag, resolveLatest, err := normalizeUpgradeTag("")
	require.NoError(t, err)
	assert.True(t, resolveLatest, "empty input should signal latest-tag resolution")
	assert.Empty(t, tag)
}

func TestNormalizeUpgradeTagPrefixesV(t *testing.T) {
	tag, resolveLatest, err := normalizeUpgradeTag("0.1.0")
	require.NoError(t, err)
	assert.False(t, resolveLatest)
	assert.Equal(t, "v0.1.0", tag)
}

func TestNormalizeUpgradeTagPreservesV(t *testing.T) {
	tag, resolveLatest, err := normalizeUpgradeTag("v0.1.0")
	require.NoError(t, err)
	assert.False(t, resolveLatest)
	assert.Equal(t, "v0.1.0", tag)
}

func TestNormalizeUpgradeTagTrimsWhitespace(t *testing.T) {
	// Without the trim, the v-prefix branch would produce "v 0.1.0" and the
	// GitHub URL would 404 with a confusing error.
	tag, resolveLatest, err := normalizeUpgradeTag("  v0.1.0  ")
	require.NoError(t, err)
	assert.False(t, resolveLatest)
	assert.Equal(t, "v0.1.0", tag)
}

func TestNormalizeUpgradeTagRejectsWhitespaceOnly(t *testing.T) {
	// A whitespace-only value is a user typo that previously survived the
	// empty check and produced "v " — guard it explicitly.
	for _, v := range []string{" ", "  ", "\t", "\n", " \t\n "} {
		_, _, err := normalizeUpgradeTag(v)
		require.Error(t, err, "tag %q should be rejected", v)
		assert.Contains(t, err.Error(), "must not be empty")
	}
}

// TestNormalizeUpgradeTagRejectsPathSyntax pins the validation added because
// the tag is interpolated straight into the release URL — Go transmits `..`
// path segments verbatim, so a tag carrying path syntax steers both the
// archive download and the checksums.txt fetch to the same substituted
// location, leaving the sha256 check verifying the substituted archive against
// its own checksums.
func TestNormalizeUpgradeTagRejectsPathSyntax(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"dot-dot segments", "../../other/repo/releases/download/v1"},
		{"leading slash", "/../../other/repo/v9"},
		{"separator after a version", "v1.2.3/../../x"},
		{"percent-encoded separator", "v1.2.3%2f..%2fx"},
		{"query string", "v1.2.3?x=1"},
		{"pre-release suffix hiding a path", "v1.2.3-/../../x"},
		{"not a version at all", "latest"},
		{"partial version", "v1.2"},
		{"too many fields", "v1.2.3.4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := normalizeUpgradeTag(tc.raw)
			require.Error(t, err, "tag %q must be rejected", tc.raw)
			assert.Contains(t, err.Error(), "--version")
		})
	}
}

// Real release tags — including the pre-release and build-metadata forms the
// daily-release workflow can produce — must keep working.
func TestNormalizeUpgradeTagAcceptsRealTags(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"v0.1.0", "v0.1.0"},
		{"0.1.0", "v0.1.0"},
		{"v1.2.3-rc1", "v1.2.3-rc1"},
		{"1.2.3-rc1", "v1.2.3-rc1"},
		{"v1.2.3-nightly.20260817", "v1.2.3-nightly.20260817"},
		{"v1.2.3+build.5", "v1.2.3+build.5"},
		{"  v10.20.30  ", "v10.20.30"},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			tag, resolveLatest, err := normalizeUpgradeTag(tc.raw)
			require.NoError(t, err)
			assert.False(t, resolveLatest)
			assert.Equal(t, tc.want, tag)
		})
	}
}
