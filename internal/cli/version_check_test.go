package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCheckOptedOutFlagWins(t *testing.T) {
	t.Setenv(envOptOut, "")
	assert.True(t, versionCheckOptedOut(true), "--no-version-check must opt out regardless of the env var")
}

func TestVersionCheckOptedOutEnvUnset(t *testing.T) {
	t.Setenv(envOptOut, "")
	assert.False(t, versionCheckOptedOut(false))
}

func TestVersionCheckOptedOutTruthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "True", "t"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(envOptOut, v)
			assert.True(t, versionCheckOptedOut(false), "%q should opt out", v)
		})
	}
}

func TestVersionCheckOptedOutFalseyValues(t *testing.T) {
	// "0"/"false" are explicit opt-ins; "no"/"garbage" are unparseable and
	// must fall back to keeping the advisory check enabled.
	for _, v := range []string{"0", "false", "no", "garbage"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(envOptOut, v)
			assert.False(t, versionCheckOptedOut(false), "%q should not opt out", v)
		})
	}
}
