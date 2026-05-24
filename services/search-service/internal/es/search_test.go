package es

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		page, size       int32
		wantFrom, wantSz int
	}{
		{"DefaultsApplied_ZeroInput", 0, 0, 0, 20},
		{"NegativeInput_ClampedToDefault", -3, -10, 0, 20},
		{"Page1Size50", 1, 50, 0, 50},
		{"Page3Size20_From40", 3, 20, 40, 20},
		{"OversizedPage_Capped100", 1, 250, 0, 100},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, sz := PageOffset(tc.page, tc.size)
			assert.Equal(t, tc.wantFrom, from)
			assert.Equal(t, tc.wantSz, sz)
		})
	}
}

func TestMultiMatchOrMatchAll(t *testing.T) {
	t.Parallel()

	t.Run("EmptyQuery_ReturnsMatchAll", func(t *testing.T) {
		q := MultiMatchOrMatchAll("", []string{"name"})
		_, ok := q["match_all"]
		require.True(t, ok, "expected match_all clause: %v", q)
	})

	t.Run("NonEmptyQuery_ReturnsMultiMatch", func(t *testing.T) {
		q := MultiMatchOrMatchAll("hola", []string{"name^3", "description"})
		mm, ok := q["multi_match"].(map[string]any)
		require.True(t, ok, "expected multi_match clause: %v", q)
		assert.Equal(t, "hola", mm["query"])
		assert.Equal(t, "best_fields", mm["type"])
	})
}

func TestBoolQuery(t *testing.T) {
	t.Parallel()

	t.Run("OnlyMust", func(t *testing.T) {
		q := BoolQuery([]map[string]any{{"match_all": map[string]any{}}}, nil)
		boolPart := q["bool"].(map[string]any)
		_, hasMust := boolPart["must"]
		_, hasFilter := boolPart["filter"]
		assert.True(t, hasMust)
		assert.False(t, hasFilter, "filter must be omitted when slice is nil")
	})

	t.Run("MustAndFilter", func(t *testing.T) {
		q := BoolQuery(
			[]map[string]any{{"match_all": map[string]any{}}},
			[]map[string]any{{"term": map[string]any{"status": "active"}}},
		)
		boolPart := q["bool"].(map[string]any)
		_, hasMust := boolPart["must"]
		_, hasFilter := boolPart["filter"]
		assert.True(t, hasMust)
		assert.True(t, hasFilter)
	})
}
