package cli

import (
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

	all := extractSets(info, "")
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
	only := extractSets(info, "b")
	assert.Len(t, only, 1)
	assert.Equal(t, "b", only[0].Namespace)
	assert.Equal(t, "s2", only[0].Name)
}

func TestExtractSetsHandlesEmptyOrMissing(t *testing.T) {
	assert.Empty(t, extractSets(map[string]any{}, ""))
	assert.Empty(t, extractSets(map[string]any{"namespaces": []any{}}, ""))
	// Malformed shapes should be skipped silently rather than panic.
	assert.Empty(t, extractSets(map[string]any{"namespaces": []any{map[string]any{"name": "x", "sets": "not-a-list"}}}, ""))
}
