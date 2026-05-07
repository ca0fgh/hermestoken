package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMarketplaceCredentialProbeUsesFullClaudeCodeProbeForAnthropic(t *testing.T) {
	db := setupMarketplaceFixedOrderRelayTestDB(t)
	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     10,
		VendorType:       constant.ChannelTypeAnthropic,
		APIKey:           "anthropic-marketplace-secret",
		BaseURL:          "https://anthropic.example/v1",
		Models:           []string{"claude-sonnet-4-6"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1,
		ConcurrencyLimit: 1,
	})
	require.NoError(t, err)

	var captured tokenverifier.DirectProbeRequest
	callCount := 0
	restore := stubMarketplaceCredentialDirectProbe(t, func(ctx context.Context, input tokenverifier.DirectProbeRequest) (*tokenverifier.DirectProbeResponse, error) {
		callCount++
		captured = input
		return &tokenverifier.DirectProbeResponse{
			Report: tokenverifier.ReportSummary{
				Score:          87,
				Grade:          "A",
				ProbeScore:     87,
				ProbeScoreMax:  92,
				ScoringVersion: tokenverifier.ScoringVersionV4,
			},
		}, nil
	})
	defer restore()

	require.NoError(t, RunMarketplaceCredentialProbe(context.Background(), credential.ID))

	assert.Equal(t, "https://anthropic.example", captured.BaseURL)
	assert.Equal(t, "anthropic-marketplace-secret", captured.APIKey)
	assert.Equal(t, tokenverifier.ProviderAnthropic, captured.Provider)
	assert.Equal(t, "claude-sonnet-4-6", captured.Model)
	assert.Equal(t, tokenverifier.ProbeProfileFull, captured.ProbeProfile)
	assert.Equal(t, tokenverifier.ClientProfileClaudeCode, captured.ClientProfile)
	assert.Empty(t, captured.CheckKeys)
	assert.Equal(t, 1, callCount)

	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.Equal(t, model.MarketplaceProbeStatusPassed, stored.ProbeStatus)
	assert.Equal(t, 87, stored.ProbeScore)
	assert.Equal(t, 92, stored.ProbeScoreMax)
	assert.Equal(t, "A", stored.ProbeGrade)
	assert.Equal(t, tokenverifier.ProviderAnthropic, stored.ProbeProvider)
	assert.Equal(t, tokenverifier.ProbeProfileFull, stored.ProbeProfile)
	assert.Equal(t, tokenverifier.ClientProfileClaudeCode, stored.ProbeClientProfile)
	assert.Equal(t, tokenverifier.ScoringVersionV4, stored.ProbeScoringVersion)
	assert.NotZero(t, stored.ProbeCheckedAt)
}

func TestRunMarketplaceCredentialProbeStoresRedactedFailure(t *testing.T) {
	db := setupMarketplaceFixedOrderRelayTestDB(t)
	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     10,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "openai-marketplace-secret",
		BaseURL:          "https://openai.example/v1",
		Models:           []string{"gpt-4o-mini"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1,
		ConcurrencyLimit: 1,
	})
	require.NoError(t, err)

	restore := stubMarketplaceCredentialDirectProbe(t, func(ctx context.Context, input tokenverifier.DirectProbeRequest) (*tokenverifier.DirectProbeResponse, error) {
		return nil, errors.New("probe failed for openai-marketplace-secret at https://openai.example")
	})
	defer restore()

	err = RunMarketplaceCredentialProbe(context.Background(), credential.ID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "openai-marketplace-secret")
	assert.NotContains(t, err.Error(), "https://openai.example")

	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.Equal(t, model.MarketplaceProbeStatusFailed, stored.ProbeStatus)
	assert.NotContains(t, stored.ProbeError, "openai-marketplace-secret")
	assert.NotContains(t, stored.ProbeError, "https://openai.example")
	assert.Contains(t, stored.ProbeError, "[redacted]")
	assert.NotZero(t, stored.ProbeCheckedAt)
}

func TestMarketplaceCredentialProbeTargetChangedIncludesChannelConfig(t *testing.T) {
	assert.True(t, marketplaceCredentialProbeTargetChanged([]string{"base_url"}, false))
	assert.True(t, marketplaceCredentialProbeTargetChanged([]string{"header_override"}, false))
	assert.True(t, marketplaceCredentialProbeTargetChanged([]string{"settings"}, false))
	assert.True(t, marketplaceCredentialProbeTargetChanged(nil, true))
	assert.False(t, marketplaceCredentialProbeTargetChanged([]string{"quota_limit"}, false))
}

func TestMarketplaceProbeReportScoreUsesZeroProbeScoreWhenMaxIsPresent(t *testing.T) {
	report := tokenverifier.ReportSummary{
		Score:         91,
		ProbeScore:    0,
		ProbeScoreMax: 82,
	}

	assert.Equal(t, 0, marketplaceProbeReportScore(report))
	assert.Equal(t, 82, marketplaceProbeReportScoreMax(report))
}

func stubMarketplaceCredentialDirectProbe(t *testing.T, fn marketplaceCredentialDirectProbeFunc) func() {
	t.Helper()
	original := runMarketplaceCredentialDirectProbe
	runMarketplaceCredentialDirectProbe = fn
	return func() {
		runMarketplaceCredentialDirectProbe = original
	}
}
