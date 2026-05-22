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
