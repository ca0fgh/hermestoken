package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertConvertedRatioValue(t *testing.T, converted map[string]any, field string, model string, expected float64) {
	t.Helper()

	fieldMap := valueMap(converted[field])
	require.NotNil(t, fieldMap, "expected %s to be present", field)

	rawValue, ok := fieldMap[model]
	require.True(t, ok, "expected %s to include %s", field, model)

	value, ok := asFloat64(rawValue)
	require.True(t, ok, "expected %s.%s to be numeric", field, model)
	assert.InDelta(t, expected, value, 0.000001)
}

func TestConvertModelsDevToRatioDataPrefersOfficialAnthropicPricing(t *testing.T) {
	const payload = `{
		"cheap-proxy": {
			"name": "Cheap Proxy",
			"models": {
				"claude-opus-4-7": {
					"cost": {
						"input": 0.14,
						"output": 0.7,
						"cache_read": 0.014,
						"cache_write": 0.175
					}
				},
				"claude-sonnet-4-6": {
					"cost": {
						"input": 0.14,
						"output": 0.7,
						"cache_read": 0.014,
						"cache_write": 0.175
					}
				},
				"claude-haiku-4-5": {
					"cost": {
						"input": 0.14,
						"output": 0.7,
						"cache_read": 0.014,
						"cache_write": 0.175
					}
				}
			}
		},
		"anthropic": {
			"name": "Anthropic",
			"models": {
				"claude-opus-4-7": {
					"cost": {
						"input": 5,
						"output": 25,
						"cache_read": 0.5,
						"cache_write": 6.25
					}
				},
				"claude-sonnet-4-6": {
					"cost": {
						"input": 3,
						"output": 15,
						"cache_read": 0.3,
						"cache_write": 3.75
					}
				},
				"claude-haiku-4-5": {
					"cost": {
						"input": 1,
						"output": 5,
						"cache_read": 0.1,
						"cache_write": 1.25
					}
				}
			}
		}
	}`

	converted, err := convertModelsDevToRatioData(strings.NewReader(payload))
	require.NoError(t, err)

	assertConvertedRatioValue(t, converted, "model_ratio", "claude-opus-4-7", 2.5)
	assertConvertedRatioValue(t, converted, "completion_ratio", "claude-opus-4-7", 5)
	assertConvertedRatioValue(t, converted, "cache_ratio", "claude-opus-4-7", 0.1)
	assertConvertedRatioValue(t, converted, "create_cache_ratio", "claude-opus-4-7", 1.25)

	assertConvertedRatioValue(t, converted, "model_ratio", "claude-sonnet-4-6", 1.5)
	assertConvertedRatioValue(t, converted, "completion_ratio", "claude-sonnet-4-6", 5)
	assertConvertedRatioValue(t, converted, "cache_ratio", "claude-sonnet-4-6", 0.1)
	assertConvertedRatioValue(t, converted, "create_cache_ratio", "claude-sonnet-4-6", 1.25)

	assertConvertedRatioValue(t, converted, "model_ratio", "claude-haiku-4-5", 0.5)
	assertConvertedRatioValue(t, converted, "completion_ratio", "claude-haiku-4-5", 5)
	assertConvertedRatioValue(t, converted, "cache_ratio", "claude-haiku-4-5", 0.1)
	assertConvertedRatioValue(t, converted, "create_cache_ratio", "claude-haiku-4-5", 1.25)
}
