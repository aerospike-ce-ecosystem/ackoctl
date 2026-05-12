package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSetsAcrossNamespaces(t *testing.T) {
	info := map[string]any{
		"namespaces": []any{
			map[string]any{
				"name": "test",
				"sets": []any{
					map[string]any{"name": "users", "objects": float64(123), "memUsed": float64(4096)},
					map[string]any{"name": "orders", "object_count": float64(50)},
				},
			},
			map[string]any{
				"name": "analytics",
				"sets": []any{
					map[string]any{"name": "events", "data-used-bytes": float64(99)},
				},
			},
		},
	}

	all, drifted := extractSets(info, "", nil)
	assert.False(t, drifted)
	assert.Len(t, all, 3)
	assert.Equal(t, "test", all[0].Namespace)
	assert.Equal(t, "users", all[0].Name)
	assert.Equal(t, float64(123), all[0].Objects)
	assert.Equal(t, float64(4096), all[0].MemUsed)

	// memory key fallback via object_count alias
	assert.Equal(t, "orders", all[1].Name)
	assert.Equal(t, float64(50), all[1].Objects)
	assert.Nil(t, all[1].MemUsed)

	// data-used-bytes alias
	assert.Equal(t, "events", all[2].Name)
	assert.Equal(t, float64(99), all[2].MemUsed)
}

func TestExtractSetsFilterByNamespace(t *testing.T) {
	info := map[string]any{
		"namespaces": []any{
			map[string]any{"name": "a", "sets": []any{map[string]any{"name": "s1"}}},
			map[string]any{"name": "b", "sets": []any{map[string]any{"name": "s2"}}},
		},
	}
	only, drifted := extractSets(info, "b", nil)
	assert.False(t, drifted)
	assert.Len(t, only, 1)
	assert.Equal(t, "b", only[0].Namespace)
	assert.Equal(t, "s2", only[0].Name)
}

func TestExtractSetsHandlesEmptyOrMissing(t *testing.T) {
	empty1, drifted := extractSets(map[string]any{}, "", nil)
	assert.False(t, drifted)
	assert.Empty(t, empty1)

	empty2, drifted := extractSets(map[string]any{"namespaces": []any{}}, "", nil)
	assert.False(t, drifted)
	assert.Empty(t, empty2)
}

func TestExtractSetsFlagsSchemaDrift(t *testing.T) {
	var buf bytes.Buffer
	rows, drifted := extractSets(map[string]any{
		"namespaces": []any{
			map[string]any{"name": "x", "sets": "not-a-list"},
		},
	}, "", &buf)
	assert.Empty(t, rows)
	assert.True(t, drifted, "expected drift when sets is not a list")
	assert.True(t, strings.Contains(buf.String(), "sets is not a list"), "expected warning, got %q", buf.String())
}

func TestExtractSetsDriftWithEmptyNamespacesList(t *testing.T) {
	// `namespaces: []` is a valid empty response, not drift.
	rows, drifted := extractSets(map[string]any{"namespaces": []any{}}, "", nil)
	assert.Empty(t, rows)
	assert.False(t, drifted)
}

func TestExtractSetsDriftWhenNamespacesNotAList(t *testing.T) {
	var buf bytes.Buffer
	rows, drifted := extractSets(map[string]any{"namespaces": "oops"}, "", &buf)
	assert.Empty(t, rows)
	assert.True(t, drifted)
	assert.True(t, strings.Contains(buf.String(), "namespaces` is not a list"), "expected warning, got %q", buf.String())
}
