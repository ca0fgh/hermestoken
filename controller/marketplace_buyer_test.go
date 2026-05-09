package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type marketplaceOrderListItemResponse struct {
	ID                    int     `json:"id"`
	SellerUserID          int     `json:"seller_user_id"`
	VendorType            int     `json:"vendor_type"`
	VendorNameSnapshot    string  `json:"vendor_name_snapshot"`
	Models                string  `json:"models"`
	QuotaMode             string  `json:"quota_mode"`
	QuotaLimit            int64   `json:"quota_limit"`
	TimeMode              string  `json:"time_mode"`
	TimeLimitSeconds      int64   `json:"time_limit_seconds"`
	Multiplier            float64 `json:"multiplier"`
	ConcurrencyLimit      int     `json:"concurrency_limit"`
	ListingStatus         string  `json:"listing_status"`
	ServiceStatus         string  `json:"service_status"`
	HealthStatus          string  `json:"health_status"`
	CapacityStatus        string  `json:"capacity_status"`
	RouteStatus           string  `json:"route_status"`
	RouteReason           string  `json:"route_reason"`
	RiskStatus            string  `json:"risk_status"`
	ProbeStatus           string  `json:"probe_status"`
	ProbeScore            int     `json:"probe_score"`
	ProbeScoreMax         int     `json:"probe_score_max"`
	ProbeGrade            string  `json:"probe_grade"`
	ProbeCheckedAt        int64   `json:"probe_checked_at"`
	CurrentConcurrency    int     `json:"current_concurrency"`
	FixedOrderSoldQuota   int64   `json:"fixed_order_sold_quota"`
	ActiveFixedOrderCount int64   `json:"active_fixed_order_count"`
	PricePreview          []struct {
		Model      string  `json:"model"`
		Multiplier float64 `json:"multiplier"`
		Official   struct {
			QuotaType             string  `json:"quota_type"`
			BillingMode           string  `json:"billing_mode"`
			ModelPrice            float64 `json:"model_price"`
			ModelRatio            float64 `json:"model_ratio"`
			InputPricePerMTok     float64 `json:"input_price_per_mtok"`
			OutputPricePerMTok    float64 `json:"output_price_per_mtok"`
			CacheReadPricePerMTok float64 `json:"cache_read_price_per_mtok"`
			Configured            bool    `json:"configured"`
		} `json:"official"`
		Buyer struct {
			QuotaType             string  `json:"quota_type"`
			BillingMode           string  `json:"billing_mode"`
			ModelPrice            float64 `json:"model_price"`
			ModelRatio            float64 `json:"model_ratio"`
			InputPricePerMTok     float64 `json:"input_price_per_mtok"`
			OutputPricePerMTok    float64 `json:"output_price_per_mtok"`
			CacheReadPricePerMTok float64 `json:"cache_read_price_per_mtok"`
			Configured            bool    `json:"configured"`
		} `json:"buyer"`
	} `json:"price_preview"`
}

type marketplaceOrderFilterRangesResponse struct {
	UnlimitedQuotaCount int64   `json:"unlimited_quota_count"`
	LimitedQuotaCount   int64   `json:"limited_quota_count"`
	MinQuotaLimit       int64   `json:"min_quota_limit"`
	MaxQuotaLimit       int64   `json:"max_quota_limit"`
	UnlimitedTimeCount  int64   `json:"unlimited_time_count"`
	LimitedTimeCount    int64   `json:"limited_time_count"`
	MinTimeLimitSeconds int64   `json:"min_time_limit_seconds"`
	MaxTimeLimitSeconds int64   `json:"max_time_limit_seconds"`
	MinMultiplier       float64 `json:"min_multiplier"`
	MaxMultiplier       float64 `json:"max_multiplier"`
	MinConcurrencyLimit int     `json:"min_concurrency_limit"`
	MaxConcurrencyLimit int     `json:"max_concurrency_limit"`
}

type marketplaceFixedOrderResponse struct {
	ID                      int     `json:"id"`
	BuyerUserID             int     `json:"buyer_user_id"`
	SellerUserID            int     `json:"seller_user_id"`
	CredentialID            int     `json:"credential_id"`
	PurchasedQuota          int64   `json:"purchased_quota"`
	RemainingQuota          int64   `json:"remaining_quota"`
	SpentQuota              int64   `json:"spent_quota"`
	RefundedQuota           int64   `json:"refunded_quota"`
	PurchaseProbeScore      int     `json:"purchase_probe_score"`
	PurchaseProbeScoreMax   int     `json:"purchase_probe_score_max"`
	RefundProbeScore        int     `json:"refund_probe_score"`
	RefundedAt              int64   `json:"refunded_at"`
	MultiplierSnapshot      float64 `json:"multiplier_snapshot"`
	PlatformFeeRateSnapshot float64 `json:"platform_fee_rate_snapshot"`
	OfficialPriceSnapshot   string  `json:"official_price_snapshot"`
	BuyerPriceSnapshot      string  `json:"buyer_price_snapshot"`
	ExpiresAt               int64   `json:"expires_at"`
	Status                  string  `json:"status"`
	ProbeStatus             string  `json:"probe_status"`
	ProbeScore              int     `json:"probe_score"`
	ProbeScoreMax           int     `json:"probe_score_max"`
	ProbeGrade              string  `json:"probe_grade"`
	ProbeCheckedAt          int64   `json:"probe_checked_at"`
}

type marketplacePoolModelResponse struct {
	VendorType         int     `json:"vendor_type"`
	VendorNameSnapshot string  `json:"vendor_name_snapshot"`
	Model              string  `json:"model"`
	CandidateCount     int     `json:"candidate_count"`
	LowestMultiplier   float64 `json:"lowest_multiplier"`
}

type marketplacePoolCandidateResponse struct {
	Credential  marketplaceOrderListItemResponse `json:"credential"`
	RouteScore  float64                          `json:"route_score"`
	SuccessRate float64                          `json:"success_rate"`
	LoadRatio   float64                          `json:"load_ratio"`
}

type marketplacePricingItemResponse struct {
	ModelName             string  `json:"model_name"`
	QuotaType             string  `json:"quota_type"`
	BillingMode           string  `json:"billing_mode"`
	ModelPrice            float64 `json:"model_price"`
	ModelRatio            float64 `json:"model_ratio"`
	CompletionRatio       float64 `json:"completion_ratio"`
	CacheRatio            float64 `json:"cache_ratio"`
	InputPricePerMTok     float64 `json:"input_price_per_mtok"`
	OutputPricePerMTok    float64 `json:"output_price_per_mtok"`
	CacheReadPricePerMTok float64 `json:"cache_read_price_per_mtok"`
	Configured            bool    `json:"configured"`
}

func TestMarketplacePricingIncludesConfiguredModelsWithoutChannelAbilities(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pricing", nil, 20)
	MarketplaceListPricing(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var items []marketplacePricingItemResponse
	require.NoError(t, json.Unmarshal(response.Data, &items))
	byModel := make(map[string]marketplacePricingItemResponse, len(items))
	for _, item := range items {
		byModel[item.ModelName] = item
	}

	gpt55, ok := byModel["gpt-5.5"]
	require.True(t, ok, "expected gpt-5.5 pricing to be present")
	assert.Equal(t, "ratio", gpt55.QuotaType)
	assert.Equal(t, "metered", gpt55.BillingMode)
	assert.True(t, gpt55.Configured)
	assert.InDelta(t, 2.5, gpt55.ModelRatio, 0.000001)
	assert.InDelta(t, 6, gpt55.CompletionRatio, 0.000001)
	assert.InDelta(t, 0.1, gpt55.CacheRatio, 0.000001)
	assert.InDelta(t, 5, gpt55.InputPricePerMTok, 0.000001)
	assert.InDelta(t, 30, gpt55.OutputPricePerMTok, 0.000001)
	assert.InDelta(t, 0.5, gpt55.CacheReadPricePerMTok, 0.000001)
}

func TestMarketplacePricingIsAvailableToAuthenticatedUsers(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pricing", nil, 20)
	MarketplaceListPricing(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var items []marketplacePricingItemResponse
	require.NoError(t, json.Unmarshal(response.Data, &items))
	require.NotEmpty(t, items)
}

func TestMarketplacePricingIncludesPricingCatalogModels(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	seedPricingAbility(t, db, "default", "zz-marketplace-pricing-catalog")
	model.RefreshPricing()
	t.Cleanup(model.InvalidatePricingCache)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pricing", nil, 20)
	MarketplaceListPricing(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var items []marketplacePricingItemResponse
	require.NoError(t, json.Unmarshal(response.Data, &items))
	byModel := make(map[string]marketplacePricingItemResponse, len(items))
	for _, item := range items {
		byModel[item.ModelName] = item
	}

	catalogModel, ok := byModel["zz-marketplace-pricing-catalog"]
	require.True(t, ok, "expected marketplace pricing to include pricing catalog models")
	assert.True(t, catalogModel.Configured)
}

func TestMarketplacePricingIncludesEnabledModelsWhenPricingCatalogCacheIsStale(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	model.RefreshPricing()
	t.Cleanup(model.InvalidatePricingCache)
	seedPricingAbility(t, db, "default", "zz-marketplace-enabled-cache-stale")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pricing", nil, 20)
	MarketplaceListPricing(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var items []marketplacePricingItemResponse
	require.NoError(t, json.Unmarshal(response.Data, &items))
	byModel := make(map[string]marketplacePricingItemResponse, len(items))
	for _, item := range items {
		byModel[item.ModelName] = item
	}

	_, ok := byModel["zz-marketplace-enabled-cache-stale"]
	require.True(t, ok, "expected marketplace pricing to include directly enabled models")
}

func TestBuyerMarketplaceOrderListShowsListedCredentialsIncludingSelfAndUntested(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	eligible := createHealthyMarketplaceCredential(t, db, 10, "test-key-eligible")
	untested := createMarketplaceCredentialViaController(t, 11, "test-key-untested")
	own := createMarketplaceCredentialViaController(t, buyerID, "test-key-own")
	zeroProbe := createHealthyMarketplaceCredential(t, db, 13, "test-key-zero-probe")
	disabled := createHealthyMarketplaceCredential(t, db, 12, "test-key-disabled")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", disabled.ID).
		Update("service_status", model.MarketplaceServiceStatusDisabled).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", eligible.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusPassed,
			"probe_score":      88,
			"probe_score_max":  93,
			"probe_grade":      "B",
			"probe_checked_at": int64(1710000010),
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", zeroProbe.ID).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusWarning,
			"probe_score":     0,
			"probe_score_max": 100,
			"probe_grade":     "Fail",
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/orders?model=gpt-4o-mini&quota_mode=unlimited", nil, buyerID)
	BuyerListMarketplaceOrders(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "test-key-eligible")
	assert.NotContains(t, recorder.Body.String(), "encrypted_api_key")
	assert.NotContains(t, recorder.Body.String(), "key_fingerprint")
	assert.NotContains(t, recorder.Body.String(), "base_url")

	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var items []marketplaceOrderListItemResponse
	require.NoError(t, json.Unmarshal(page.Items, &items))
	require.Len(t, items, 4)
	byID := marketplaceOrderListItemsByID(items)
	require.Contains(t, byID, eligible.ID)
	require.Contains(t, byID, untested.ID)
	require.Contains(t, byID, own.ID)
	require.Contains(t, byID, zeroProbe.ID)
	assert.Equal(t, 10, byID[eligible.ID].SellerUserID)
	assert.Equal(t, buyerID, byID[own.ID].SellerUserID)
	assert.Equal(t, constant.ChannelTypeOpenAI, byID[eligible.ID].VendorType)
	assert.Equal(t, model.MarketplaceHealthStatusHealthy, byID[eligible.ID].HealthStatus)
	assert.Equal(t, model.MarketplaceHealthStatusUntested, byID[untested.ID].HealthStatus)
	assert.Equal(t, model.MarketplaceHealthStatusUntested, byID[own.ID].HealthStatus)
	assert.Equal(t, model.MarketplaceRouteStatusAvailable, byID[eligible.ID].RouteStatus)
	assert.Equal(t, model.MarketplaceRouteStatusFailed, byID[untested.ID].RouteStatus)
	assert.Equal(t, model.MarketplaceRouteReasonProbeInProgress, byID[untested.ID].RouteReason)
	assert.Equal(t, model.MarketplaceRouteStatusFailed, byID[own.ID].RouteStatus)
	assert.Equal(t, model.MarketplaceRouteReasonProbeInProgress, byID[own.ID].RouteReason)
	assert.Equal(t, model.MarketplaceRouteStatusFailed, byID[zeroProbe.ID].RouteStatus)
	assert.Equal(t, model.MarketplaceRouteReasonProbeScoreZero, byID[zeroProbe.ID].RouteReason)
	assert.Equal(t, model.MarketplaceTimeModeLimited, byID[eligible.ID].TimeMode)
	assert.Equal(t, int64(3600), byID[eligible.ID].TimeLimitSeconds)
	assert.Equal(t, model.MarketplaceProbeStatusPassed, byID[eligible.ID].ProbeStatus)
	assert.Equal(t, 88, byID[eligible.ID].ProbeScore)
	assert.Equal(t, 93, byID[eligible.ID].ProbeScoreMax)
	assert.Equal(t, "B", byID[eligible.ID].ProbeGrade)
	assert.Equal(t, int64(1710000010), byID[eligible.ID].ProbeCheckedAt)
	require.NotEmpty(t, byID[eligible.ID].PricePreview)
	assert.Equal(t, "gpt-4o-mini", byID[eligible.ID].PricePreview[0].Model)
	assert.Equal(t, 1.25, byID[eligible.ID].PricePreview[0].Multiplier)
	if byID[eligible.ID].PricePreview[0].Official.QuotaType == "price" {
		assert.InDelta(t, byID[eligible.ID].PricePreview[0].Official.ModelPrice*1.25, byID[eligible.ID].PricePreview[0].Buyer.ModelPrice, 0.000001)
	} else {
		assert.InDelta(t, byID[eligible.ID].PricePreview[0].Official.ModelRatio*1.25, byID[eligible.ID].PricePreview[0].Buyer.ModelRatio, 0.000001)
		assert.Equal(t, "metered", byID[eligible.ID].PricePreview[0].Official.BillingMode)
		assert.InDelta(t, 0.15, byID[eligible.ID].PricePreview[0].Official.InputPricePerMTok, 0.000001)
		assert.InDelta(t, 0.6, byID[eligible.ID].PricePreview[0].Official.OutputPricePerMTok, 0.000001)
		assert.InDelta(t, 0.075, byID[eligible.ID].PricePreview[0].Official.CacheReadPricePerMTok, 0.000001)
		assert.InDelta(t, 0.1875, byID[eligible.ID].PricePreview[0].Buyer.InputPricePerMTok, 0.000001)
		assert.InDelta(t, 0.75, byID[eligible.ID].PricePreview[0].Buyer.OutputPricePerMTok, 0.000001)
		assert.InDelta(t, 0.09375, byID[eligible.ID].PricePreview[0].Buyer.CacheReadPricePerMTok, 0.000001)
	}
}

func TestBuyerMarketplaceOrderFilterRangesUseEligibleListedOrders(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)

	createHealthyMarketplaceCredential(t, db, 10, "range-key-unlimited-quota")
	limitedLow := createHealthyMarketplaceCredential(t, db, 11, "range-key-limited-low")
	limitedHigh := createHealthyMarketplaceCredential(t, db, 12, "range-key-limited-high")
	unlimitedTime := createHealthyMarketplaceCredential(t, db, 13, "range-key-unlimited-time")
	excluded := createHealthyMarketplaceCredential(t, db, 14, "range-key-excluded")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", limitedLow.ID).
		Updates(map[string]any{
			"quota_mode":         model.MarketplaceQuotaModeLimited,
			"quota_limit":        1000,
			"time_mode":          model.MarketplaceTimeModeLimited,
			"time_limit_seconds": 600,
			"multiplier":         1.1,
			"concurrency_limit":  1,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", limitedHigh.ID).
		Updates(map[string]any{
			"quota_mode":         model.MarketplaceQuotaModeLimited,
			"quota_limit":        5000,
			"time_mode":          model.MarketplaceTimeModeLimited,
			"time_limit_seconds": 7200,
			"multiplier":         2.25,
			"concurrency_limit":  6,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", unlimitedTime.ID).
		Updates(map[string]any{
			"time_mode":          model.MarketplaceTimeModeUnlimited,
			"time_limit_seconds": 0,
			"multiplier":         1.75,
			"concurrency_limit":  4,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", excluded.ID).
		Updates(map[string]any{
			"quota_mode":      model.MarketplaceQuotaModeLimited,
			"quota_limit":     1,
			"service_status":  model.MarketplaceServiceStatusDisabled,
			"capacity_status": model.MarketplaceCapacityStatusExhausted,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/order-filter-ranges?quota_mode=limited&time_mode=limited", nil, buyerID)
	BuyerGetMarketplaceOrderFilterRanges(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var ranges marketplaceOrderFilterRangesResponse
	require.NoError(t, json.Unmarshal(response.Data, &ranges))
	assert.Equal(t, int64(2), ranges.UnlimitedQuotaCount)
	assert.Equal(t, int64(2), ranges.LimitedQuotaCount)
	assert.Equal(t, int64(1000), ranges.MinQuotaLimit)
	assert.Equal(t, int64(5000), ranges.MaxQuotaLimit)
	assert.Equal(t, int64(1), ranges.UnlimitedTimeCount)
	assert.Equal(t, int64(3), ranges.LimitedTimeCount)
	assert.Equal(t, int64(600), ranges.MinTimeLimitSeconds)
	assert.Equal(t, int64(7200), ranges.MaxTimeLimitSeconds)
	assert.InDelta(t, 1.1, ranges.MinMultiplier, 0.000001)
	assert.InDelta(t, 2.25, ranges.MaxMultiplier, 0.000001)
	assert.Equal(t, 1, ranges.MinConcurrencyLimit)
	assert.Equal(t, 6, ranges.MaxConcurrencyLimit)
}

func TestBuyerMarketplaceOrderListFiltersByOrderRanges(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)

	low := createHealthyMarketplaceCredential(t, db, 10, "range-list-low")
	mid := createHealthyMarketplaceCredential(t, db, 11, "range-list-mid")
	high := createHealthyMarketplaceCredential(t, db, 12, "range-list-high")
	unlimited := createHealthyMarketplaceCredential(t, db, 13, "range-list-unlimited")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", low.ID).
		Updates(map[string]any{
			"quota_mode":         model.MarketplaceQuotaModeLimited,
			"quota_limit":        1000,
			"time_mode":          model.MarketplaceTimeModeLimited,
			"time_limit_seconds": 600,
			"multiplier":         1.1,
			"concurrency_limit":  1,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", mid.ID).
		Updates(map[string]any{
			"quota_mode":         model.MarketplaceQuotaModeLimited,
			"quota_limit":        3000,
			"time_mode":          model.MarketplaceTimeModeLimited,
			"time_limit_seconds": 3600,
			"multiplier":         1.5,
			"concurrency_limit":  3,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", high.ID).
		Updates(map[string]any{
			"quota_mode":         model.MarketplaceQuotaModeLimited,
			"quota_limit":        6000,
			"time_mode":          model.MarketplaceTimeModeLimited,
			"time_limit_seconds": 7200,
			"multiplier":         2.5,
			"concurrency_limit":  6,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", unlimited.ID).
		Updates(map[string]any{
			"quota_mode": model.MarketplaceQuotaModeUnlimited,
			"time_mode":  model.MarketplaceTimeModeUnlimited,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/orders?min_quota_limit=2000&max_quota_limit=5000&min_time_limit_seconds=1800&max_time_limit_seconds=5400&min_multiplier=1.2&max_multiplier=2&min_concurrency_limit=2&max_concurrency_limit=4", nil, buyerID)
	BuyerListMarketplaceOrders(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var items []marketplaceOrderListItemResponse
	require.NoError(t, json.Unmarshal(page.Items, &items))
	byID := marketplaceOrderListItemsByID(items)
	assert.NotContains(t, byID, low.ID)
	assert.Contains(t, byID, mid.ID)
	assert.NotContains(t, byID, high.ID)
	assert.NotContains(t, byID, unlimited.ID)
}

func TestBuyerMarketplaceOrderListExcludesStatsExhaustedLimitedCredentials(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	available := createHealthyMarketplaceCredential(t, db, 10, "stats-available")
	exhausted := createHealthyMarketplaceCredential(t, db, 11, "stats-exhausted")
	staleAvailable := createHealthyMarketplaceCredential(t, db, 12, "stats-stale-available")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id IN ?", []int{available.ID, staleAvailable.ID}).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusPassed,
			"probe_score":     90,
			"probe_score_max": 100,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", exhausted.ID).
		Updates(map[string]any{
			"quota_mode":      model.MarketplaceQuotaModeLimited,
			"quota_limit":     100000,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", exhausted.ID).
		Update("quota_used", 100000).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", staleAvailable.ID).
		Updates(map[string]any{
			"quota_mode":      model.MarketplaceQuotaModeLimited,
			"quota_limit":     100000,
			"capacity_status": model.MarketplaceCapacityStatusExhausted,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/orders?model=gpt-4o-mini", nil, buyerID)
	BuyerListMarketplaceOrders(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var items []marketplaceOrderListItemResponse
	require.NoError(t, json.Unmarshal(page.Items, &items))
	byID := marketplaceOrderListItemsByID(items)
	require.Contains(t, byID, available.ID)
	require.Contains(t, byID, staleAvailable.ID)
	assert.NotContains(t, byID, exhausted.ID)
	assert.Equal(t, model.MarketplaceCapacityStatusAvailable, byID[staleAvailable.ID].CapacityStatus)
	assert.Equal(t, model.MarketplaceRouteStatusAvailable, byID[staleAvailable.ID].RouteStatus)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsStatsExhaustedCredential(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-stats-exhausted")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"quota_mode":      model.MarketplaceQuotaModeLimited,
			"quota_limit":     100000,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", credential.ID).
		Update("quota_used", 100000).Error)

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not eligible")

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)
}

func TestBuyerMarketplaceFixedOrderPurchaseEscrowsQuotaAndKeepsListingAvailable(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 2500,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "test-key-seller")
	assert.NotContains(t, recorder.Body.String(), "encrypted_api_key")

	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))
	assert.Equal(t, buyerID, order.BuyerUserID)
	assert.Equal(t, 10, order.SellerUserID)
	assert.Equal(t, credential.ID, order.CredentialID)
	assert.Equal(t, int64(2500), order.PurchasedQuota)
	assert.Equal(t, int64(2500), order.RemainingQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusActive, order.Status)
	assert.Equal(t, 1.25, order.MultiplierSnapshot)
	assert.Equal(t, 0.0, order.PlatformFeeRateSnapshot)
	assert.Equal(t, 90, order.PurchaseProbeScore)
	assert.Equal(t, 100, order.PurchaseProbeScoreMax)
	assert.Contains(t, order.OfficialPriceSnapshot, "gpt-4o-mini")
	assert.Contains(t, order.BuyerPriceSnapshot, "gpt-4o-mini")
	assert.Greater(t, order.ExpiresAt, int64(0))

	var buyer model.User
	require.NoError(t, db.First(&buyer, buyerID).Error)
	assert.Equal(t, 7500, buyer.Quota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, db.First(&stats, "credential_id = ?", credential.ID).Error)
	assert.Equal(t, int64(2500), stats.FixedOrderSoldQuota)
	assert.Equal(t, int64(1), stats.ActiveFixedOrderCount)

	ctx, listRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/orders", nil, buyerID)
	BuyerListMarketplaceOrders(ctx)
	listResponse := decodeAPIResponse(t, listRecorder)
	require.True(t, listResponse.Success, listResponse.Message)
	assert.Contains(t, listRecorder.Body.String(), fmt.Sprintf(`"id":%d`, credential.ID))
}

func TestBuyerMarketplaceFixedOrderPurchaseSnapshotsProbeScore(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-probe-snapshot")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusPassed,
			"probe_score":      91,
			"probe_score_max":  96,
			"probe_grade":      "A",
			"probe_checked_at": int64(1710000040),
		}).Error)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		"/api/marketplace/fixed-orders",
		map[string]any{
			"credential_id":   credential.ID,
			"purchased_quota": 2500,
		},
		buyerID,
	)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))
	assert.Equal(t, 91, order.PurchaseProbeScore)
	assert.Equal(t, 96, order.PurchaseProbeScoreMax)
	assert.Equal(t, model.MarketplaceProbeStatusPassed, order.ProbeStatus)
	assert.Equal(t, 91, order.ProbeScore)
	assert.Equal(t, 96, order.ProbeScoreMax)
}

func TestBuyerMarketplaceFixedOrderPurchaseUsesUnlimitedSellerTimeCondition(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-unlimited-time")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"time_mode":          model.MarketplaceTimeModeUnlimited,
			"time_limit_seconds": 0,
		}).Error)

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 2500,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))
	assert.Equal(t, int64(0), order.ExpiresAt)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsInsufficientQuotaWithoutSideEffects(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 1000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 2500,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "insufficient")

	var buyer model.User
	require.NoError(t, db.First(&buyer, buyerID).Error)
	assert.Equal(t, 1000, buyer.Quota)

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, db.First(&stats, "credential_id = ?", credential.ID).Error)
	assert.Equal(t, int64(0), stats.FixedOrderSoldQuota)
	assert.Equal(t, int64(0), stats.ActiveFixedOrderCount)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsUntestedCredential(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createMarketplaceCredentialViaController(t, 10, "test-key-untested")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not eligible")
	assert.NotContains(t, recorder.Body.String(), "test-key-untested")

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsZeroProbeScore(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-zero-probe-purchase")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusWarning,
			"probe_score":     0,
			"probe_score_max": 100,
			"probe_grade":     "Fail",
		}).Error)

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not eligible")

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsFailedCredential(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-failed")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Update("health_status", model.MarketplaceHealthStatusFailed).Error)

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not eligible")
}

func TestBuyerMarketplaceFixedOrderPurchaseAcceptsUSDAmount(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-usd")

	body := map[string]any{
		"credential_id":        credential.ID,
		"purchased_amount_usd": 0.01,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	expectedQuota := int64(common.QuotaPerUnit / 100)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))
	assert.Equal(t, expectedQuota, order.PurchasedQuota)
	assert.Equal(t, expectedQuota, order.RemainingQuota)

	var buyer model.User
	require.NoError(t, db.First(&buyer, buyerID).Error)
	assert.Equal(t, 10000-int(expectedQuota), buyer.Quota)
}

func TestBuyerMarketplaceFixedOrderPurchaseUSDAmountIncludesTransactionFee(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 20000000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-usd-fee")
	originalFeeRate := setting.MarketplaceFeeRate
	originalMaxFixedOrderQuota := setting.MarketplaceMaxFixedOrderQuota
	setting.MarketplaceFeeRate = 0.05
	setting.MarketplaceMaxFixedOrderQuota = 20000000
	t.Cleanup(func() {
		setting.MarketplaceFeeRate = originalFeeRate
		setting.MarketplaceMaxFixedOrderQuota = originalMaxFixedOrderQuota
	})

	body := map[string]any{
		"credential_id":        credential.ID,
		"purchased_amount_usd": 30.0,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	baseQuota := int64(30 * common.QuotaPerUnit)
	expectedBuyerCharge := baseQuota + int64(1.5*common.QuotaPerUnit)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))
	assert.Equal(t, expectedBuyerCharge, order.PurchasedQuota)
	assert.Equal(t, expectedBuyerCharge, order.RemainingQuota)
	assert.Equal(t, 0.05, order.PlatformFeeRateSnapshot)

	var buyer model.User
	require.NoError(t, db.First(&buyer, buyerID).Error)
	assert.Equal(t, 20000000-int(expectedBuyerCharge), buyer.Quota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, db.First(&stats, "credential_id = ?", credential.ID).Error)
	assert.Equal(t, expectedBuyerCharge, stats.FixedOrderSoldQuota)
	assert.Equal(t, int64(1), stats.ActiveFixedOrderCount)
	assert.Equal(t, int64(15750000), expectedBuyerCharge)
}

func TestBuyerMarketplaceFixedOrderPurchaseRejectsOwnCredential(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, buyerID, "test-key-own")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "cannot buy own")
	assert.NotContains(t, recorder.Body.String(), "test-key-own")

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)

	var buyer model.User
	require.NoError(t, db.First(&buyer, buyerID).Error)
	assert.Equal(t, 10000, buyer.Quota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, db.First(&stats, "credential_id = ?", credential.ID).Error)
	assert.Equal(t, int64(0), stats.FixedOrderSoldQuota)
	assert.Equal(t, int64(0), stats.ActiveFixedOrderCount)
}

func TestBuyerMarketplacePoolModelsExcludeUntestedAndOwnListedCredentials(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	createMarketplaceCredentialViaController(t, 10, "pool-key-untested")
	createMarketplaceCredentialViaController(t, buyerID, "pool-key-own")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pool/models?vendor_type=1", nil, buyerID)
	BuyerListMarketplacePoolModels(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var models []marketplacePoolModelResponse
	require.NoError(t, json.Unmarshal(response.Data, &models))
	require.Empty(t, models)
}

func TestBuyerMarketplaceFixedOrderListAndDetailAreOwnerScoped(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	otherBuyerID := 21
	seedMarketplaceUser(t, db, buyerID, 10000)
	seedMarketplaceUser(t, db, otherBuyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusWarning,
			"probe_score":      64,
			"probe_score_max":  82,
			"probe_grade":      "C",
			"probe_checked_at": int64(1710000020),
		}).Error)

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))

	ctx, listRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/fixed-orders", nil, buyerID)
	BuyerListMarketplaceFixedOrders(ctx)
	listResponse := decodeAPIResponse(t, listRecorder)
	require.True(t, listResponse.Success, listResponse.Message)
	assert.Contains(t, listRecorder.Body.String(), fmt.Sprintf(`"id":%d`, order.ID))
	assert.NotContains(t, listRecorder.Body.String(), "base_url")
	assert.NotContains(t, listRecorder.Body.String(), "key_fingerprint")
	var listPage marketplacePageResponse
	require.NoError(t, json.Unmarshal(listResponse.Data, &listPage))
	var listedOrders []marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(listPage.Items, &listedOrders))
	require.Len(t, listedOrders, 1)
	assert.Equal(t, model.MarketplaceProbeStatusWarning, listedOrders[0].ProbeStatus)
	assert.Equal(t, 64, listedOrders[0].ProbeScore)
	assert.Equal(t, 82, listedOrders[0].ProbeScoreMax)

	ctx, detailRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/fixed-orders/1", nil, buyerID)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}}
	BuyerGetMarketplaceFixedOrder(ctx)
	detailResponse := decodeAPIResponse(t, detailRecorder)
	require.True(t, detailResponse.Success, detailResponse.Message)
	var detail marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(detailResponse.Data, &detail))
	assert.Equal(t, model.MarketplaceProbeStatusWarning, detail.ProbeStatus)
	assert.Equal(t, 64, detail.ProbeScore)
	assert.Equal(t, 82, detail.ProbeScoreMax)

	ctx, otherRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/fixed-orders/1", nil, otherBuyerID)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}}
	BuyerGetMarketplaceFixedOrder(ctx)
	otherResponse := decodeAPIResponse(t, otherRecorder)
	require.False(t, otherResponse.Success)
}

func TestBuyerBindMarketplaceFixedOrderToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	token := seedToken(t, db, buyerID, "buyer-token", "bind1234token5678")
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))

	bindCtx, bindRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/marketplace/fixed-orders/%d/bind-token", order.ID),
		map[string]any{"token_id": token.Id},
		buyerID,
	)
	bindCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(order.ID)}}
	BuyerBindMarketplaceFixedOrderToken(bindCtx)
	bindResponse := decodeAPIResponse(t, bindRecorder)
	require.True(t, bindResponse.Success, bindResponse.Message)

	var detail tokenResponseItem
	require.NoError(t, json.Unmarshal(bindResponse.Data, &detail))
	assert.Equal(t, token.Id, detail.ID)
	assert.Equal(t, order.ID, detail.MarketplaceFixedOrderID)
	assert.NotContains(t, bindRecorder.Body.String(), token.Key)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.Equal(t, order.ID, stored.MarketplaceFixedOrderID)
	assert.Equal(t, []int{order.ID}, stored.MarketplaceFixedOrderIDList())
}

func TestBuyerEditMarketplaceFixedOrderTokenBindingsSupportsMultipleOrders(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	token := seedToken(t, db, buyerID, "buyer-token", "multi1234token5678")
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	first := createMarketplaceFixedOrderForBindingTest(t, buyerID, credential.ID)
	second := createMarketplaceFixedOrderForBindingTest(t, buyerID, credential.ID)

	bindCtx, bindRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		"/api/marketplace/fixed-orders/bind-token",
		map[string]any{
			"token_id":        token.Id,
			"fixed_order_ids": []int{first.ID, second.ID},
		},
		buyerID,
	)
	BuyerBindMarketplaceFixedOrderToken(bindCtx)
	bindResponse := decodeAPIResponse(t, bindRecorder)
	require.True(t, bindResponse.Success, bindResponse.Message)

	var detail tokenResponseItem
	require.NoError(t, json.Unmarshal(bindResponse.Data, &detail))
	assert.Equal(t, first.ID, detail.MarketplaceFixedOrderID)
	assert.Equal(t, []int{first.ID, second.ID}, detail.MarketplaceFixedOrderIDs)
	assert.NotContains(t, bindRecorder.Body.String(), token.Key)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.Equal(t, first.ID, stored.MarketplaceFixedOrderID)
	assert.Equal(t, []int{first.ID, second.ID}, stored.MarketplaceFixedOrderIDList())
}

func TestBuyerBindMarketplaceFixedOrderTokensSupportsMultipleTokens(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	firstToken := seedToken(t, db, buyerID, "buyer-token-a", "multiatoken123456")
	secondToken := seedToken(t, db, buyerID, "buyer-token-b", "multibtoken123456")
	removedToken := seedToken(t, db, buyerID, "buyer-token-c", "multictoken123456")
	otherBuyerToken := seedToken(t, db, 21, "other-token", "otherbuyer123456")
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	firstOrder := createMarketplaceFixedOrderForBindingTest(t, buyerID, credential.ID)
	secondOrder := createMarketplaceFixedOrderForBindingTest(t, buyerID, credential.ID)

	firstToken.SetMarketplaceFixedOrderIDList([]int{secondOrder.ID})
	require.NoError(t, firstToken.Update())
	removedToken.SetMarketplaceFixedOrderIDList([]int{firstOrder.ID})
	require.NoError(t, removedToken.Update())
	otherBuyerToken.SetMarketplaceFixedOrderIDList([]int{firstOrder.ID})
	require.NoError(t, otherBuyerToken.Update())

	bindCtx, bindRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/marketplace/fixed-orders/%d/bind-tokens", firstOrder.ID),
		map[string]any{
			"token_ids": []int{firstToken.Id, secondToken.Id},
		},
		buyerID,
	)
	bindCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(firstOrder.ID)}}
	BuyerBindMarketplaceFixedOrderTokens(bindCtx)
	bindResponse := decodeAPIResponse(t, bindRecorder)
	require.True(t, bindResponse.Success, bindResponse.Message)
	assert.NotContains(t, bindRecorder.Body.String(), firstToken.Key)
	assert.NotContains(t, bindRecorder.Body.String(), secondToken.Key)

	var response struct {
		FixedOrderID int                 `json:"fixed_order_id"`
		TokenIDs     []int               `json:"token_ids"`
		Tokens       []tokenResponseItem `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(bindResponse.Data, &response))
	assert.Equal(t, firstOrder.ID, response.FixedOrderID)
	assert.Equal(t, []int{firstToken.Id, secondToken.Id}, response.TokenIDs)
	require.Len(t, response.Tokens, 2)
	assert.Equal(t, firstToken.Id, response.Tokens[0].ID)
	assert.Equal(t, secondToken.Id, response.Tokens[1].ID)

	var storedFirst model.Token
	require.NoError(t, db.First(&storedFirst, firstToken.Id).Error)
	assert.Equal(t, []int{firstOrder.ID, secondOrder.ID}, storedFirst.MarketplaceFixedOrderIDList())

	var storedSecond model.Token
	require.NoError(t, db.First(&storedSecond, secondToken.Id).Error)
	assert.Equal(t, []int{firstOrder.ID}, storedSecond.MarketplaceFixedOrderIDList())

	var storedRemoved model.Token
	require.NoError(t, db.First(&storedRemoved, removedToken.Id).Error)
	assert.Empty(t, storedRemoved.MarketplaceFixedOrderIDList())

	var storedOther model.Token
	require.NoError(t, db.First(&storedOther, otherBuyerToken.Id).Error)
	assert.Equal(t, []int{firstOrder.ID}, storedOther.MarketplaceFixedOrderIDList())
}

func TestBuyerBindMarketplaceFixedOrderTokenRejectsUnownedToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	otherBuyerID := 21
	seedMarketplaceUser(t, db, buyerID, 10000)
	seedMarketplaceUser(t, db, otherBuyerID, 10000)
	token := seedToken(t, db, otherBuyerID, "other-token", "other1234token5678")
	credential := createHealthyMarketplaceCredential(t, db, 10, "test-key-seller")

	body := map[string]any{
		"credential_id":   credential.ID,
		"purchased_quota": 1000,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/fixed-orders", body, buyerID)
	BuyerCreateMarketplaceFixedOrder(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var order marketplaceFixedOrderResponse
	require.NoError(t, json.Unmarshal(response.Data, &order))

	bindCtx, bindRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/marketplace/fixed-orders/%d/bind-token", order.ID),
		map[string]any{"token_id": token.Id},
		buyerID,
	)
	bindCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(order.ID)}}
	BuyerBindMarketplaceFixedOrderToken(bindCtx)
	bindResponse := decodeAPIResponse(t, bindRecorder)
	require.False(t, bindResponse.Success)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.Equal(t, 0, stored.MarketplaceFixedOrderID)
}

func createMarketplaceFixedOrderForBindingTest(t *testing.T, buyerID int, credentialID int) *model.MarketplaceFixedOrder {
	t.Helper()
	order, err := service.CreateMarketplaceFixedOrder(service.MarketplaceFixedOrderCreateInput{
		BuyerUserID:    buyerID,
		CredentialID:   credentialID,
		PurchasedQuota: 1000,
	})
	require.NoError(t, err)
	return order
}

func TestBuyerMarketplacePoolModelsAggregateEligibleModels(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	first := createHealthyMarketplaceCredential(t, db, 10, "pool-key-one")
	second := createHealthyMarketplaceCredential(t, db, 11, "pool-key-two")
	disabled := createHealthyMarketplaceCredential(t, db, 12, "pool-key-disabled")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id IN ?", []int{first.ID, second.ID}).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusPassed,
			"probe_score":     90,
			"probe_score_max": 100,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", second.ID).
		Update("multiplier", 1.1).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", disabled.ID).
		Update("service_status", model.MarketplaceServiceStatusDisabled).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pool/models?vendor_type=1", nil, buyerID)
	BuyerListMarketplacePoolModels(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "pool-key-one")
	assert.NotContains(t, recorder.Body.String(), "encrypted_api_key")

	var models []marketplacePoolModelResponse
	require.NoError(t, json.Unmarshal(response.Data, &models))
	require.Len(t, models, 1)
	assert.Equal(t, first.VendorType, models[0].VendorType)
	assert.Equal(t, "gpt-4o-mini", models[0].Model)
	assert.Equal(t, 2, models[0].CandidateCount)
	assert.Equal(t, 1.1, models[0].LowestMultiplier)
}

func TestBuyerMarketplacePoolCandidatesExcludeUnavailableAndSortByRoutePriority(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	available := createHealthyMarketplaceCredential(t, db, 10, "pool-key-available")
	lowerPriority := createHealthyMarketplaceCredential(t, db, 13, "pool-key-lower-priority")
	unscored := createHealthyMarketplaceCredential(t, db, 14, "pool-key-unscored")
	busy := createHealthyMarketplaceCredential(t, db, 11, "pool-key-busy")
	exhausted := createHealthyMarketplaceCredential(t, db, 12, "pool-key-exhausted")
	own := createHealthyMarketplaceCredential(t, db, buyerID, "pool-key-own")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", available.ID).
		Update("multiplier", 1.05).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", lowerPriority.ID).
		Update("multiplier", 1.8).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", own.ID).
		Update("multiplier", 0.5).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", available.ID).
		Updates(map[string]any{
			"success_count":        9,
			"upstream_error_count": 1,
			"avg_latency_ms":       120,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", available.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusPassed,
			"probe_score":      95,
			"probe_score_max":  100,
			"probe_grade":      "A",
			"probe_checked_at": int64(1710000030),
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", lowerPriority.ID).
		Updates(map[string]any{
			"success_count":        8,
			"upstream_error_count": 2,
			"avg_latency_ms":       500,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", lowerPriority.ID).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusWarning,
			"probe_score":     70,
			"probe_score_max": 100,
			"probe_grade":     "C",
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", unscored.ID).
		Updates(map[string]any{
			"probe_status":    model.MarketplaceProbeStatusUnscored,
			"probe_score":     0,
			"probe_score_max": 0,
			"probe_grade":     "",
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", busy.ID).
		Update("current_concurrency", 2).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", exhausted.ID).
		Updates(map[string]any{
			"quota_mode":  model.MarketplaceQuotaModeLimited,
			"quota_limit": 1000,
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", exhausted.ID).
		Update("quota_used", 1000).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/pool/candidates?model=gpt-4o-mini", nil, buyerID)
	BuyerListMarketplacePoolCandidates(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "pool-key-available")
	assert.NotContains(t, recorder.Body.String(), "encrypted_api_key")
	assert.NotContains(t, recorder.Body.String(), "key_fingerprint")
	assert.NotContains(t, recorder.Body.String(), "base_url")

	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var candidates []marketplacePoolCandidateResponse
	require.NoError(t, json.Unmarshal(page.Items, &candidates))
	require.Len(t, candidates, 2)
	assert.Equal(t, available.ID, candidates[0].Credential.ID)
	assert.Equal(t, lowerPriority.ID, candidates[1].Credential.ID)

	candidatesByID := make(map[int]marketplacePoolCandidateResponse, len(candidates))
	for _, candidate := range candidates {
		candidatesByID[candidate.Credential.ID] = candidate
	}
	require.Contains(t, candidatesByID, available.ID)
	require.Contains(t, candidatesByID, lowerPriority.ID)
	require.NotContains(t, candidatesByID, unscored.ID)
	require.NotContains(t, candidatesByID, own.ID)
	assert.Greater(t, candidatesByID[available.ID].RouteScore, 0.0)
	assert.Equal(t, model.MarketplaceProbeStatusPassed, candidatesByID[available.ID].Credential.ProbeStatus)
	assert.Equal(t, 95, candidatesByID[available.ID].Credential.ProbeScore)
	assert.Equal(t, 100, candidatesByID[available.ID].Credential.ProbeScoreMax)
	assert.InDelta(t, 0.9, candidatesByID[available.ID].SuccessRate, 0.000001)
	assert.Equal(t, 0.0, candidatesByID[available.ID].LoadRatio)
	assert.Equal(t, 1.05, candidatesByID[available.ID].Credential.Multiplier)
	assert.Equal(t, 2, candidatesByID[available.ID].Credential.ConcurrencyLimit)
	assert.Equal(t, 0, candidatesByID[available.ID].Credential.CurrentConcurrency)
}

func TestBuyerSaveMarketplacePoolFiltersStoresTokenScopedFilters(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	token := seedToken(t, db, buyerID, "pool-filter-token", "poolfiltertoken123")

	body := map[string]any{
		"token_id": token.Id,
		"filters": map[string]any{
			"vendor_type":            constant.ChannelTypeOpenAI,
			"model":                  "gpt-4o-mini",
			"max_multiplier":         1.2,
			"min_concurrency_limit":  2,
			"max_concurrency_limit":  5,
			"min_time_limit_seconds": 60,
			"max_time_limit_seconds": 3600,
			"min_quota_limit":        100,
			"max_quota_limit":        1000,
			"quota_mode":             model.MarketplaceQuotaModeLimited,
			"time_mode":              model.MarketplaceTimeModeLimited,
		},
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/pool/token-filters", body, buyerID)
	BuyerSaveMarketplacePoolFilters(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, "id = ? AND user_id = ?", token.Id, buyerID).Error)
	require.True(t, stored.MarketplacePoolFiltersEnabled)
	saved := stored.MarketplacePoolFilters.Values()
	assert.Equal(t, constant.ChannelTypeOpenAI, saved.VendorType)
	assert.Equal(t, "gpt-4o-mini", saved.Model)
	assert.Equal(t, 1.2, saved.MaxMultiplier)
	assert.Equal(t, 2, saved.MinConcurrencyLimit)
	assert.Equal(t, model.MarketplaceQuotaModeLimited, saved.QuotaMode)
	assert.Equal(t, model.MarketplaceTimeModeLimited, saved.TimeMode)
}

func TestBuyerResetMarketplacePoolFiltersClearsTokenScopedFilters(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	buyerID := 20
	seedMarketplaceUser(t, db, buyerID, 10000)
	token := seedToken(t, db, buyerID, "pool-filter-reset-token", "poolfilterreset123")
	token.MarketplaceRouteEnabled = model.NewMarketplaceRouteEnabled([]string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})
	token.MarketplacePoolFiltersEnabled = true
	token.MarketplacePoolFilters = model.NewMarketplacePoolFilters(model.MarketplacePoolFilterValues{
		Model:         "gpt-4o-mini",
		MaxMultiplier: 1.2,
	})
	require.NoError(t, token.Update())

	body := map[string]any{"token_id": token.Id}
	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/marketplace/pool/token-filters", body, buyerID)
	BuyerResetMarketplacePoolFilters(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, "id = ? AND user_id = ?", token.Id, buyerID).Error)
	require.False(t, stored.MarketplacePoolFiltersEnabled)
	assert.Empty(t, stored.MarketplacePoolFilters.Values())
	assert.Equal(t, []string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool}, stored.MarketplaceRouteEnabledList())
}

func TestBuyerSaveMarketplacePoolFiltersRejectsUnownedToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	seedMarketplaceUser(t, db, 20, 10000)
	seedMarketplaceUser(t, db, 21, 10000)
	token := seedToken(t, db, 21, "other-pool-token", "otherpooltoken123")

	body := map[string]any{
		"token_id": token.Id,
		"filters":  map[string]any{"max_multiplier": 1.2},
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/pool/token-filters", body, 20)
	BuyerSaveMarketplacePoolFilters(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.False(t, stored.MarketplacePoolFiltersEnabled)
	assert.Empty(t, stored.MarketplacePoolFilters.Values())
}

func seedMarketplaceUser(t *testing.T, db *gorm.DB, userID int, quota int) {
	t.Helper()

	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: fmt.Sprintf("marketplace_user_%d", userID),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    quota,
		AffCode:  fmt.Sprintf("aff_%d", userID),
	}).Error)
}

func createHealthyMarketplaceCredential(t *testing.T, db *gorm.DB, sellerID int, apiKey string) marketplaceCredentialResponse {
	t.Helper()

	credential := createMarketplaceCredentialViaController(t, sellerID, apiKey)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
			"probe_status":    model.MarketplaceProbeStatusPassed,
			"probe_score":     90,
			"probe_score_max": 100,
		}).Error)
	credential.HealthStatus = model.MarketplaceHealthStatusHealthy
	credential.CapacityStatus = model.MarketplaceCapacityStatusAvailable
	credential.RiskStatus = model.MarketplaceRiskStatusNormal
	credential.ProbeStatus = model.MarketplaceProbeStatusPassed
	credential.ProbeScore = 90
	credential.ProbeScoreMax = 100
	return credential
}

func marketplaceOrderListItemsByID(items []marketplaceOrderListItemResponse) map[int]marketplaceOrderListItemResponse {
	byID := make(map[int]marketplaceOrderListItemResponse, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	return byID
}
