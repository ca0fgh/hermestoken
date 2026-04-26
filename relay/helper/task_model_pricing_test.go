package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetTaskModelPricing(t *testing.T) {
	t.Helper()
	require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(`{}`))
}

func TestApplyTaskModelPricingCombinesPerRequestAndPerSecond(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })
	resetTaskModelPricing(t)
	t.Cleanup(func() { resetTaskModelPricing(t) })

	require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(`{
		"veo_3_1": {"per_request": 0.2, "per_second": 0.6}
	}`))

	priceData := types.PriceData{
		ModelPrice: 0.6,
		UsePrice:   true,
		Quota:      600,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 2,
		},
		OtherRatios: map[string]float64{
			"seconds":    8,
			"resolution": 1.5,
		},
	}

	applied := ApplyTaskModelPricing("veo_3_1", &priceData)

	assert.True(t, applied)
	assert.Equal(t, 15000, priceData.Quota)
	assert.True(t, priceData.TaskModelPricingApplied)
	assert.Equal(t, 0.2, priceData.TaskPerRequestPrice)
	assert.Equal(t, 0.6, priceData.TaskPerSecondPrice)
}

func TestApplyTaskModelPricingCanIgnoreNonTimeRatios(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })
	resetTaskModelPricing(t)
	t.Cleanup(func() { resetTaskModelPricing(t) })

	require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(`{
		"veo_3_1": {"per_request": 0.2, "per_second": 0.6, "include_other_ratios": false}
	}`))

	priceData := types.PriceData{
		ModelPrice: 0.6,
		UsePrice:   true,
		Quota:      600,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 2,
		},
		OtherRatios: map[string]float64{
			"seconds":    8,
			"resolution": 1.5,
		},
	}

	applied := ApplyTaskModelPricing("veo_3_1", &priceData)

	assert.True(t, applied)
	assert.Equal(t, 10000, priceData.Quota)
}

func TestApplyTaskModelPricingLeavesLegacyPricingUntouched(t *testing.T) {
	resetTaskModelPricing(t)
	t.Cleanup(func() { resetTaskModelPricing(t) })

	priceData := types.PriceData{
		ModelPrice: 0.6,
		UsePrice:   true,
		Quota:      600,
		OtherRatios: map[string]float64{
			"seconds": 8,
		},
	}

	applied := ApplyTaskModelPricing("veo_3_1", &priceData)

	assert.False(t, applied)
	assert.Equal(t, 600, priceData.Quota)
	assert.False(t, priceData.TaskModelPricingApplied)
}
