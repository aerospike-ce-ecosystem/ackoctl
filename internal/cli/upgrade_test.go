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
