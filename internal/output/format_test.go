package output

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleConn struct {
	Name   string `yaml:"name" json:"name"`
	Host   string `yaml:"host" json:"host"`
	Port   int    `yaml:"port" json:"port"`
	Hidden string `yaml:"-" json:"-"`
}

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]Format{"": FormatTable, "table": FormatTable, "json": FormatJSON, "YAML": FormatYAML, "yml": FormatYAML} {
		got, err := Parse(in)
		require.NoError(t, err)
		assert.Equal(t, want, got, "input %q", in)
	}
	_, err := Parse("xml")
	assert.Error(t, err)
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatJSON, sampleConn{Name: "kind", Host: "h", Port: 3000}))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "kind", got["name"])
	assert.NotContains(t, got, "Hidden")
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatYAML, sampleConn{Name: "kind", Host: "h", Port: 3000}))
	out := buf.String()
	assert.Contains(t, out, "name: kind")
	assert.Contains(t, out, "port: 3000")
	assert.NotContains(t, out, "Hidden")
}

func TestPrintTableExplicit(t *testing.T) {
	var buf bytes.Buffer
	rows := []sampleConn{{Name: "a", Host: "h1", Port: 1}, {Name: "b", Host: "h2", Port: 2}}
	err := Print(&buf, FormatTable, rows,
		WithTable(
			[]string{"NAME", "HOST", "PORT"},
			func(v any) []string {
				c := v.(sampleConn)
				return []string{c.Name, c.Host, strconv.Itoa(c.Port)}
			},
			func(v any) []any {
				out := []any{}
				for _, r := range v.([]sampleConn) {
					out = append(out, r)
				}
				return out
			},
		),
	)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[1], "a")
	assert.Contains(t, lines[2], "b")
}

func TestPrintTableFallback(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatTable, sampleConn{Name: "x", Host: "y", Port: 9}))
	out := buf.String()
	assert.Contains(t, out, "name:")
	assert.Contains(t, out, "x")
	assert.NotContains(t, out, "Hidden")
}

type withPointers struct {
	Enabled *bool   `yaml:"enabled" json:"enabled"`
	Count   *int    `yaml:"count" json:"count"`
	Tag     *string `yaml:"tag" json:"tag"`
}

func TestPrintTableDereferencesPointers(t *testing.T) {
	var buf bytes.Buffer
	yes, n := true, 7
	require.NoError(t, Print(&buf, FormatTable, withPointers{Enabled: &yes, Count: &n, Tag: nil}))
	out := buf.String()
	assert.Regexp(t, `enabled:\s+true`, out)
	assert.Regexp(t, `count:\s+7`, out)
	assert.NotContains(t, out, "0x", "pointer should be dereferenced, not printed as an address")
	assert.Regexp(t, `tag:\s*$`, out)
}

func TestPrintTableNestedRawMap(t *testing.T) {
	var buf bytes.Buffer
	info := map[string]any{
		"name": "BB9",
		"namespaces": []any{
			map[string]any{
				"name":    "test",
				"objects": float64(12),
				"sets":    []any{map[string]any{"name": "users"}},
			},
		},
		"nodes": []any{},
	}
	require.NoError(t, Print(&buf, FormatTable, info))
	out := buf.String()
	assert.Contains(t, out, "name:")
	assert.Contains(t, out, "BB9")
	assert.Contains(t, out, "namespaces:")
	assert.Contains(t, out, "test")
	// keys must be alphabetically ordered so output is stable
	assert.Less(t, strings.Index(out, "name:"), strings.Index(out, "namespaces:"))
	// empty slice should not crash and should render as [] on the value column
	assert.Regexp(t, `nodes:\s+\[\]`, out)
}

