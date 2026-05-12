package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHostsTrimsAndFilters(t *testing.T) {
	got, err := sanitizeHosts([]string{" a.example ", "", "b.example"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.example", "b.example"}, got)
}

func TestSanitizeHostsRejectsAllEmpty(t *testing.T) {
	_, err := sanitizeHosts([]string{"", "  "})
	require.Error(t, err)
}

func TestSanitizeHostsNilPassthrough(t *testing.T) {
	got, err := sanitizeHosts(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}
