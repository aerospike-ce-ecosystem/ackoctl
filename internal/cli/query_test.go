package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJSONScalarBareword(t *testing.T) {
	v, err := parseJSONScalar("alice")
	require.NoError(t, err)
	assert.Equal(t, "alice", v)
}

func TestParseJSONScalarNumber(t *testing.T) {
	v, err := parseJSONScalar("30")
	require.NoError(t, err)
	assert.Equal(t, float64(30), v)
}

func TestParseJSONScalarList(t *testing.T) {
	v, err := parseJSONScalar(`[1,2,3]`)
	require.NoError(t, err)
	assert.Equal(t, []any{float64(1), float64(2), float64(3)}, v)
}

func TestParseJSONScalarQuotedString(t *testing.T) {
	v, err := parseJSONScalar(`"alice"`)
	require.NoError(t, err)
	assert.Equal(t, "alice", v)
}

func TestParseJSONScalarRejectsTruncatedList(t *testing.T) {
	_, err := parseJSONScalar("[1,2")
	require.Error(t, err, "truncated JSON list must not fall back to a plain string predicate")
	assert.Contains(t, err.Error(), "looks like JSON")
}

func TestParseJSONScalarRejectsBrokenObject(t *testing.T) {
	_, err := parseJSONScalar(`{"bad`)
	require.Error(t, err)
}

func TestParseJSONScalarNullLiteral(t *testing.T) {
	v, err := parseJSONScalar("null")
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestParseJSONScalarBoolLiteral(t *testing.T) {
	v, err := parseJSONScalar("true")
	require.NoError(t, err)
	assert.Equal(t, true, v)
}

func TestParseJSONScalarNegativeNumber(t *testing.T) {
	v, err := parseJSONScalar("-3.14")
	require.NoError(t, err)
	assert.Equal(t, -3.14, v)
}
