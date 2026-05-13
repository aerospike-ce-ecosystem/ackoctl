package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntFieldFromFloat64(t *testing.T) {
	v, ok := intField(map[string]any{"size": float64(3)}, "size")
	assert.True(t, ok)
	assert.Equal(t, 3, v)
}

func TestIntFieldFromInt(t *testing.T) {
	v, ok := intField(map[string]any{"size": 5}, "size")
	assert.True(t, ok)
	assert.Equal(t, 5, v)
}

func TestIntFieldMissingKey(t *testing.T) {
	_, ok := intField(map[string]any{}, "size")
	assert.False(t, ok)
}

func TestIntFieldNilValue(t *testing.T) {
	_, ok := intField(map[string]any{"size": nil}, "size")
	assert.False(t, ok)
}

func TestIntFieldUnexpectedType(t *testing.T) {
	_, ok := intField(map[string]any{"size": "five"}, "size")
	assert.False(t, ok, "string value should not be coerced to int")
}
