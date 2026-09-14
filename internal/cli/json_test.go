package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSONNumberKeepsInt64Precision(t *testing.T) {
	m := map[string]any{}
	require.NoError(t, decodeJSONNumber(`{"id":1234567890123456789,"age":30}`, &m))
	assert.Equal(t, json.Number("1234567890123456789"), m["id"])
	assert.Equal(t, json.Number("30"), m["age"])

	// json.Number re-marshals verbatim, which is the property the fix relies on.
	out, err := json.Marshal(m)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":1234567890123456789,"age":30}`, string(out))
	assert.Contains(t, string(out), "1234567890123456789")
}

func TestDecodeJSONNumberRejectsTrailingData(t *testing.T) {
	// json.Unmarshal rejects trailing data; a bare json.Decoder would stop at
	// the first value and silently ignore the rest, so the helper must not
	// loosen that guarantee.
	for _, raw := range []string{`[1,2] junk`, `{"a":1}{"b":2}`, `30 40`} {
		var v any
		require.Error(t, decodeJSONNumber(raw, &v), "trailing data must be rejected: %s", raw)
	}
}

func TestDecodeJSONNumberAllowsSurroundingWhitespace(t *testing.T) {
	var v any
	require.NoError(t, decodeJSONNumber("  30\n", &v))
	assert.Equal(t, json.Number("30"), v)
}

func TestCompareJSONNumbers(t *testing.T) {
	cases := []struct {
		a, b       string
		want       int
		comparable bool
	}{
		{"10", "20", -1, true},
		{"10", "10", 0, true},
		{"20", "10", 1, true},
		// Beyond float64's 53-bit mantissa these two collapse onto the same
		// float64, so the int64 path is what keeps the ordering exact.
		{"1234567890123456789", "1234567890123456788", 1, true},
		{"1e3", "5", 1, true},
		{"10.5", "10.1", 1, true},
	}
	for _, tc := range cases {
		got, ok := compareJSONNumbers(json.Number(tc.a), json.Number(tc.b))
		assert.Equal(t, tc.comparable, ok, "%s vs %s", tc.a, tc.b)
		assert.Equal(t, tc.want, got, "%s vs %s", tc.a, tc.b)
	}
}
