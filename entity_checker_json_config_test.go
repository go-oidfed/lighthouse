package lighthouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityCheckerFromJSONConfig_TrustPathStringAnchors(t *testing.T) {
	config := map[string]any{
		"trust_anchors": []any{"https://ta.example.org", "https://ta2.example.org"},
	}
	checker, err := EntityCheckerFromJSONConfig("trust_path", config)
	require.NoError(t, err)

	tp, ok := checker.(*TrustPathEntityChecker)
	require.True(t, ok)
	assert.Equal(t, []string{"https://ta.example.org", "https://ta2.example.org"}, tp.TrustAnchorIDs)
}

func TestEntityCheckerFromJSONConfig_TrustPathObjectAnchors(t *testing.T) {
	config := map[string]any{
		"trust_anchors": []any{
			map[string]any{"entity_id": "https://ta.oidf-pilot.edugain.org"},
		},
	}
	checker, err := EntityCheckerFromJSONConfig("trust_path", config)
	require.NoError(t, err)

	tp, ok := checker.(*TrustPathEntityChecker)
	require.True(t, ok)
	assert.Equal(t, []string{"https://ta.oidf-pilot.edugain.org"}, tp.TrustAnchorIDs)
}

func TestEntityCheckerFromJSONConfig_TrustMarkObjectAnchors(t *testing.T) {
	config := map[string]any{
		"trust_mark_type": "https://tm.example.org",
		"trust_anchors": []any{
			map[string]any{"entity_id": "https://ta.example.org"},
		},
	}
	checker, err := EntityCheckerFromJSONConfig("trust_mark", config)
	require.NoError(t, err)

	tm, ok := checker.(*TrustMarkEntityChecker)
	require.True(t, ok)
	assert.Equal(t, "https://tm.example.org", tm.TrustMarkType)
	assert.Equal(t, []string{"https://ta.example.org"}, tm.TrustAnchorIDs)
}

func TestEntityCheckerFromJSONConfig_MultipleOrObjectAnchors(t *testing.T) {
	config := []any{
		map[string]any{
			"type": "trust_path",
			"config": map[string]any{
				"trust_anchors": []any{
					map[string]any{"entity_id": "https://ta.example.org"},
				},
			},
		},
		map[string]any{
			"type": "entity_id",
			"config": map[string]any{
				"entity_ids": []any{"https://op.example.org"},
			},
		},
	}
	checker, err := EntityCheckerFromJSONConfig("multiple_or", config)
	require.NoError(t, err)

	mo, ok := checker.(*MultipleEntityCheckerOr)
	require.True(t, ok)
	require.Len(t, mo.Checkers, 2)
	tp, ok := mo.Checkers[0].(*TrustPathEntityChecker)
	require.True(t, ok)
	assert.Equal(t, []string{"https://ta.example.org"}, tp.TrustAnchorIDs)
}

func TestEntityCheckerFromJSONConfig_UnknownType(t *testing.T) {
	_, err := EntityCheckerFromJSONConfig("no_such_checker", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown entity check type")
}
