package service

import (
	"testing"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMarketplaceConcurrencyAllowsUnlimited(t *testing.T) {
	originalMaxConcurrency := setting.MarketplaceMaxCredentialConcurrency
	t.Cleanup(func() { setting.MarketplaceMaxCredentialConcurrency = originalMaxConcurrency })

	setting.MarketplaceMaxCredentialConcurrency = 5

	normalized, err := normalizeMarketplaceConcurrency(0)
	require.NoError(t, err)
	assert.Equal(t, 0, normalized)

	_, err = normalizeMarketplaceConcurrency(-1)
	require.Error(t, err)

	_, err = normalizeMarketplaceConcurrency(6)
	require.Error(t, err)

	setting.MarketplaceMaxCredentialConcurrency = 0
	normalized, err = normalizeMarketplaceConcurrency(1000)
	require.NoError(t, err)
	assert.Equal(t, 1000, normalized)
}

func TestCreateMarketplaceCredentialDefaultsMissingConcurrencyToOne(t *testing.T) {
	db := setupMarketplaceFixedOrderRelayTestDB(t)

	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID: 1,
		VendorType:   1,
		APIKey:       "marketplace-default-concurrency-key",
		Models:       []string{"gpt-4o-mini"},
		QuotaMode:    model.MarketplaceQuotaModeUnlimited,
		Multiplier:   1,
	})
	require.NoError(t, err)

	var saved model.MarketplaceCredential
	require.NoError(t, db.First(&saved, credential.ID).Error)
	assert.Equal(t, 1, saved.ConcurrencyLimit)
}

func TestCreateMarketplaceCredentialAllowsExplicitUnlimitedConcurrency(t *testing.T) {
	db := setupMarketplaceFixedOrderRelayTestDB(t)
	unlimitedConcurrency := 0

	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     1,
		VendorType:       1,
		APIKey:           "marketplace-unlimited-concurrency-key",
		Models:           []string{"gpt-4o-mini"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1,
		ConcurrencyLimit: &unlimitedConcurrency,
	})
	require.NoError(t, err)

	var saved model.MarketplaceCredential
	require.NoError(t, db.First(&saved, credential.ID).Error)
	assert.Equal(t, 0, saved.ConcurrencyLimit)
}

func TestMarketplacePoolUnlimitedConcurrencyIsEligibleAndUnloaded(t *testing.T) {
	credential := model.MarketplaceCredential{
		ListingStatus:    model.MarketplaceListingStatusListed,
		ServiceStatus:    model.MarketplaceServiceStatusEnabled,
		HealthStatus:     model.MarketplaceHealthStatusHealthy,
		RiskStatus:       model.MarketplaceRiskStatusNormal,
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		ConcurrencyLimit: 0,
		Multiplier:       1,
	}
	stats := model.MarketplaceCredentialStats{
		CurrentConcurrency: 500,
	}

	assert.True(t, isMarketplacePoolCredentialEligible(credential, stats))
	assert.Equal(t, 0.0, marketplacePoolLoadRatio(credential, stats))
}
