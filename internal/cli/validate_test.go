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

func TestValidateBinsJSONObjectRejectsEmptyObject(t *testing.T) {
	// `{}` is a syntactically valid JSON object but semantically empty — the
	// server has no use for a put with zero bins, and the docstring on
	// validateJSONObjectFlag promises a non-empty object. Guard against the
	// zero-length map slipping through to the wire.
	for _, v := range []string{`{}`, `  {}  `, "\n{}\n"} {
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

func TestValidateJSONObjectFlagAccepted(t *testing.T) {
	for _, v := range []string{
		`{"a":1}`,
		`{}`, // shape check only; emptiness is enforced by validateBinsJSONObject
		`  {"a":"b","c":[1,2]}  `,
		"\n{\"x\":true}\n",
	} {
		require.NoError(t, validateJSONObjectFlag(v, "--filter"), "value %q should be accepted", v)
	}
}

func TestValidateJSONObjectFlagRejectsMalformedBrace(t *testing.T) {
	// Regression: these all start with '{' but are not valid JSON. A previous
	// "first byte only" check accepted them because it never parsed the body,
	// letting malformed input round-trip to the server as an opaque 422.
	for _, v := range []string{
		`{"a":}`,
		`{bad`,
		`{"a":1`,
		`{,}`,
		`{"a":1,}`,
	} {
		err := validateJSONObjectFlag(v, "--filter")
		require.Error(t, err, "malformed value %q should be rejected", v)
		assert.Contains(t, err.Error(), "--filter must be a JSON object")
	}
}

func TestValidateJSONObjectFlagRejectsNonObject(t *testing.T) {
	cases := []struct {
		in   string
		kind string
	}{
		{`[1,2,3]`, "array"},
		{`"hi"`, "string"},
		{`7`, "number"},
		{`null`, "null"},
		{`false`, "bool"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			err := validateJSONObjectFlag(tc.in, "--predicate")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--predicate must be a JSON object")
			assert.Contains(t, err.Error(), tc.kind)
		})
	}
}

func TestValidateJSONObjectFlagRejectsEmpty(t *testing.T) {
	for _, v := range []string{"", " ", "\t\n  "} {
		err := validateJSONObjectFlag(v, "--filter")
		require.Error(t, err, "value %q should be rejected", v)
		assert.Contains(t, err.Error(), "non-empty JSON object")
	}
}

func TestValidateBinsJSONObjectRejectsMalformedBrace(t *testing.T) {
	// The --bins wrapper delegates the shape check to validateJSONObjectFlag,
	// so brace-prefixed malformed JSON must now be caught at the validator
	// layer rather than relying on a later json.Unmarshal in the call site.
	for _, v := range []string{`{"a":}`, `{bad`, `{"a":1`} {
		err := validateBinsJSONObject(v)
		require.Error(t, err, "malformed bins %q should be rejected", v)
		assert.Contains(t, err.Error(), "--bins must be a JSON object")
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

func TestValidateColorAccepted(t *testing.T) {
	// Empty means "not supplied" and a well-formed #RRGGBB triplet (either
	// case) are the only accepted shapes.
	for _, v := range []string{"", "#1E88E5", "#abcdef", "#ABCDEF", "#000000", "#ffffff"} {
		require.NoError(t, validateColor(v), "color %q should be accepted", v)
	}
}

func TestValidateColorRejectsMalformed(t *testing.T) {
	cases := []string{
		"blue",       // bareword, no leading '#'
		"1E88E5",     // missing leading '#'
		"#FFF",       // 3-digit shorthand the UI does not render
		"#1E88E50",   // too long
		"#GGGGGG",    // non-hex digits
		"#12345g",    // trailing non-hex digit
		"#",          // just the hash
		"  #1E88E5 ", // surrounding whitespace is not trimmed for a hex value
	}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			err := validateColor(v)
			require.Error(t, err, "color %q should be rejected", v)
			assert.Contains(t, err.Error(), "#RRGGBB")
		})
	}
}

func TestCleanStringSlice(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil stays nil", nil, nil},
		{"empty stays nil", []string{}, nil},
		{"trims surrounding whitespace", []string{" read ", "\twrite\n"}, []string{"read", "write"}},
		{"drops blank entry from trailing comma", []string{"read", ""}, []string{"read"}},
		{"drops whitespace-only entries", []string{"read", "   "}, []string{"read"}},
		{"all-blank collapses to nil", []string{"", "  ", "\t"}, nil},
		{"preserves order and clean entries", []string{"10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cleanStringSlice(tc.in))
		})
	}
}
