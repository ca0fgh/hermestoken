package setting

import (
	"testing"

	"github.com/ca0fgh/hermestoken/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceSettingsDefaultsAreSafe(t *testing.T) {
	assert.False(t, MarketplaceEnabled)
	assert.Empty(t, MarketplaceEnabledVendorTypes)
	assert.Equal(t, 0.0, MarketplaceFeeRate)
	assert.Equal(t, 0, MarketplaceMinFixedOrderQuota)
	assert.Equal(t, 0, MarketplaceMaxFixedOrderQuota)
	assert.Equal(t, 30*24*60*60, MarketplaceFixedOrderDefaultExpirySeconds)
	assert.Equal(t, 10.0, MarketplaceMaxSellerMultiplier)
	assert.Equal(t, 5, MarketplaceMaxCredentialConcurrency)
}

func TestUpdateMarketplaceEnabledVendorTypesByJSONString(t *testing.T) {
	original := append([]int(nil), MarketplaceEnabledVendorTypes...)
	t.Cleanup(func() { MarketplaceEnabledVendorTypes = original })

	err := UpdateMarketplaceEnabledVendorTypesByJSONString(`[1,14]`)
	require.NoError(t, err)
	assert.True(t, IsMarketplaceVendorTypeEnabled(constant.ChannelTypeOpenAI))
	assert.True(t, IsMarketplaceVendorTypeEnabled(constant.ChannelTypeAnthropic))
	assert.False(t, IsMarketplaceVendorTypeEnabled(constant.ChannelTypeMidjourney))
	assert.JSONEq(t, `[1,14]`, MarketplaceEnabledVendorTypesToJSONString())
}

func TestMarketplaceEnabledVendorTypesToJSONStringUsesEmptyArray(t *testing.T) {
	original := append([]int(nil), MarketplaceEnabledVendorTypes...)
	MarketplaceEnabledVendorTypes = nil
	t.Cleanup(func() { MarketplaceEnabledVendorTypes = original })

	assert.JSONEq(t, `[]`, MarketplaceEnabledVendorTypesToJSONString())
}

func TestUpdateMarketplaceEnabledVendorTypesRejectsInvalidJSON(t *testing.T) {
	original := append([]int(nil), MarketplaceEnabledVendorTypes...)
	t.Cleanup(func() { MarketplaceEnabledVendorTypes = original })

	err := UpdateMarketplaceEnabledVendorTypesByJSONString(`{"bad":true}`)
	require.Error(t, err)
	assert.Equal(t, original, MarketplaceEnabledVendorTypes)
}
