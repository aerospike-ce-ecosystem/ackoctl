package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePKTypeAccepted(t *testing.T) {
	for _, v := range []string{"", "auto", "string", "int", "bytes"} {
		require.NoError(t, validatePKType(v), "pk-type %q should be accepted", v)
	}
}

func TestValidatePKTypeRejectsUnknown(t *testing.T) {
	for _, v := range []string{"integer", "str", "Auto", "blob"} {
		err := validatePKType(v)
		require.Error(t, err, "pk-type %q should be rejected", v)
		assert.Contains(t, err.Error(), "auto|string|int|bytes")
	}
}

func TestValidateIndexTypeAccepted(t *testing.T) {
	for _, v := range []string{"numeric", "string", "geo2dsphere"} {
		require.NoError(t, validateIndexType(v), "index type %q should be accepted", v)
	}
}

func TestValidateIndexTypeRejectsUnknownAndEmpty(t *testing.T) {
	for _, v := range []string{"", "geo", "int", "GEO2DSPHERE"} {
		err := validateIndexType(v)
		require.Error(t, err, "index type %q should be rejected", v)
		assert.Contains(t, err.Error(), "numeric|string|geo2dsphere")
	}
}

func TestValidatePortAccepted(t *testing.T) {
	for _, p := range []int{1, 3000, 8080, 65535} {
		require.NoError(t, validatePort(p), "port %d should be accepted", p)
	}
}

func TestValidatePortRejectsOutOfRange(t *testing.T) {
	for _, p := range []int{0, -1, 65536, 100000} {
		err := validatePort(p)
		require.Error(t, err, "port %d should be rejected", p)
		assert.Contains(t, err.Error(), "between 1 and 65535")
	}
}

func TestValidateQueryOpAccepted(t *testing.T) {
	for _, v := range []string{"", "equals", "between", "contains", "geo_within_region", "geo_contains_point"} {
		require.NoError(t, validateQueryOp(v), "op %q should be accepted", v)
	}
}

func TestValidateQueryOpRejectsUnknown(t *testing.T) {
	for _, v := range []string{"equal", "EQUALS", "eq", "geo", "in"} {
		err := validateQueryOp(v)
		require.Error(t, err, "op %q should be rejected", v)
		assert.Contains(t, err.Error(), "equals|between|contains|geo_within_region|geo_contains_point")
	}
}

func TestValidateBinsJSONObjectAccepted(t *testing.T) {
	for _, v := range []string{
		`{"foo":1}`,
		`{}`,
		`  {"foo":"bar","baz":[1,2,3]}  `,
		"\n{\"a\":1}\n",
	} {
		require.NoError(t, validateBinsJSONObject(v), "bins %q should be accepted", v)
	}
}

func TestValidateBinsJSONObjectRejectsEmpty(t *testing.T) {
	for _, v := range []string{"", " ", "\t\n  "} {
		err := validateBinsJSONObject(v)
		require.Error(t, err, "bins %q should be rejected", v)
		assert.Contains(t, err.Error(), "non-empty JSON object")
	}
}

func TestValidateBinsJSONObjectRejectsNonObject(t *testing.T) {
	cases := []struct {
		in   string
		kind string
	}{
		{`[1,2,3]`, "array"},
		{`"hello"`, "string"},
		{`42`, "number"},
		{`null`, "null"},
		{`true`, "bool"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			err := validateBinsJSONObject(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "JSON object")
			assert.Contains(t, err.Error(), tc.kind)
		})
	}
}

func TestValidateBinsJSONObjectRejectsMalformed(t *testing.T) {
	// Leading char is not '{' and the body is not valid JSON either — the
	// underlying decoder error should still flow through wrapped, not be
	// swallowed by the kind-detection fallthrough.
	err := validateBinsJSONObject("not json at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON object")
}

func TestValidatePKMatchModeAccepted(t *testing.T) {
	for _, v := range []string{"", "exact", "prefix", "regex"} {
		require.NoError(t, validatePKMatchMode(v), "pk-match-mode %q should be accepted", v)
	}
}

func TestValidatePKMatchModeRejectsUnknown(t *testing.T) {
	for _, v := range []string{"prefex", "Exact", "glob", "substring"} {
		err := validatePKMatchMode(v)
		require.Error(t, err, "pk-match-mode %q should be rejected", v)
		assert.Contains(t, err.Error(), "exact|prefix|regex")
	}
}
