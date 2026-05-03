package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplacePoolFiltersJSONRoundTrip(t *testing.T) {
	var filters MarketplacePoolFilters
	require.NoError(t, json.Unmarshal([]byte(`{
		"vendor_type": 1,
		"model": "gpt-4o-mini",
		"quota_mode": "unlimited",
		"time_mode": "limited",
		"min_time_limit_seconds": 60,
		"max_time_limit_seconds": 3600,
		"min_multiplier": 1.1,
		"max_multiplier": 1.5,
		"min_concurrency_limit": 2,
		"max_concurrency_limit": 5
	}`), &filters))

	values := filters.Values()
	assert.Equal(t, 1, values.VendorType)
	assert.Equal(t, "gpt-4o-mini", values.Model)
	assert.Equal(t, MarketplaceQuotaModeUnlimited, values.QuotaMode)
	assert.Equal(t, MarketplaceTimeModeLimited, values.TimeMode)
	assert.Equal(t, int64(60), values.MinTimeLimitSeconds)
	assert.Equal(t, int64(3600), values.MaxTimeLimitSeconds)
	assert.Equal(t, 1.1, values.MinMultiplier)
	assert.Equal(t, 1.5, values.MaxMultiplier)
	assert.Equal(t, 2, values.MinConcurrencyLimit)
	assert.Equal(t, 5, values.MaxConcurrencyLimit)

	payload, err := json.Marshal(filters)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"vendor_type":1`)
	assert.Contains(t, string(payload), `"model":"gpt-4o-mini"`)
}

func TestMarketplacePoolFiltersAcceptsStoredJSONString(t *testing.T) {
	var filters MarketplacePoolFilters
	require.NoError(t, json.Unmarshal([]byte(`"{\"vendor_type\":2,\"max_multiplier\":1.2}"`), &filters))

	values := filters.Values()
	assert.Equal(t, 2, values.VendorType)
	assert.Equal(t, 1.2, values.MaxMultiplier)
}

func TestMarketplacePoolFiltersNormalizesInvalidValues(t *testing.T) {
	filters := NewMarketplacePoolFilters(MarketplacePoolFilterValues{
		VendorType:          -1,
		QuotaMode:           "bad-mode",
		TimeMode:            MarketplaceTimeModeUnlimited,
		MinQuotaLimit:       -100,
		MaxMultiplier:       -1,
		MinConcurrencyLimit: -2,
	})

	values := filters.Values()
	assert.Zero(t, values.VendorType)
	assert.Empty(t, values.QuotaMode)
	assert.Equal(t, MarketplaceTimeModeUnlimited, values.TimeMode)
	assert.Zero(t, values.MinQuotaLimit)
	assert.Zero(t, values.MaxMultiplier)
	assert.Zero(t, values.MinConcurrencyLimit)
}
