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
