package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type marketplaceCredentialResponse struct {
	ID                 int     `json:"id"`
	SellerUserID       int     `json:"seller_user_id"`
	VendorType         int     `json:"vendor_type"`
	VendorNameSnapshot string  `json:"vendor_name_snapshot"`
	KeyFingerprint     string  `json:"key_fingerprint"`
	OpenAIOrganization string  `json:"openai_organization"`
	TestModel          string  `json:"test_model"`
	BaseURL            string  `json:"base_url"`
	Other              string  `json:"other"`
	ModelMapping       string  `json:"model_mapping"`
	StatusCodeMapping  string  `json:"status_code_mapping"`
	Setting            string  `json:"setting"`
	ParamOverride      string  `json:"param_override"`
	HeaderOverride     string  `json:"header_override"`
	OtherSettings      string  `json:"settings"`
	Models             string  `json:"models"`
	QuotaMode          string  `json:"quota_mode"`
	QuotaLimit         int64   `json:"quota_limit"`
	TimeMode           string  `json:"time_mode"`
	TimeLimitSeconds   int64   `json:"time_limit_seconds"`
	Multiplier         float64 `json:"multiplier"`
	ConcurrencyLimit   int     `json:"concurrency_limit"`
	ListingStatus      string  `json:"listing_status"`
	ServiceStatus      string  `json:"service_status"`
	HealthStatus       string  `json:"health_status"`
	CapacityStatus     string  `json:"capacity_status"`
	RouteStatus        string  `json:"route_status"`
	RouteReason        string  `json:"route_reason"`
	RiskStatus         string  `json:"risk_status"`
	ProbeStatus        string  `json:"probe_status"`
	ProbeScore         int     `json:"probe_score"`
	ProbeScoreMax      int     `json:"probe_score_max"`
	ProbeGrade         string  `json:"probe_grade"`
	ProbeCheckedAt     int64   `json:"probe_checked_at"`
	ResponseTime       int     `json:"response_time"`
	TestTime           int64   `json:"test_time"`
	CurrentConcurrency int     `json:"current_concurrency"`
	QuotaUsed          int64   `json:"quota_used"`
}

type marketplacePageResponse struct {
	Items    json.RawMessage `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

func setupMarketplaceSellerControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}
	originalMarketplaceEnabled := setting.MarketplaceEnabled
	originalVendorTypes := append([]int(nil), setting.MarketplaceEnabledVendorTypes...)
	originalFeeRate := setting.MarketplaceFeeRate
	originalMinFixedOrderQuota := setting.MarketplaceMinFixedOrderQuota
	originalMaxFixedOrderQuota := setting.MarketplaceMaxFixedOrderQuota
	originalFixedOrderExpiry := setting.MarketplaceFixedOrderDefaultExpirySeconds
	originalMaxMultiplier := setting.MarketplaceMaxSellerMultiplier
	originalMaxConcurrency := setting.MarketplaceMaxCredentialConcurrency
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	originalModelRatio := ratio_setting.ModelRatio2JSONString()
	originalTaskPricing := ratio_setting.TaskModelPricing2JSONString()
	originalCompletionRatio := ratio_setting.CompletionRatio2JSONString()
	originalCacheRatio := ratio_setting.CacheRatio2JSONString()
	originalCreateCacheRatio := ratio_setting.CreateCacheRatio2JSONString()
	originalImageRatio := ratio_setting.ImageRatio2JSONString()
	originalAudioRatio := ratio_setting.AudioRatio2JSONString()
	originalAudioCompletionRatio := ratio_setting.AudioCompletionRatio2JSONString()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	ratio_setting.InitRatioSettings()
	service.InitHttpClient()
	model.InitColumnMetadata()
	t.Setenv("MARKETPLACE_CREDENTIAL_SECRET", "0123456789abcdef0123456789abcdef")

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Option{},
		&model.MarketplaceCredential{},
		&model.MarketplaceCredentialStats{},
		&model.MarketplaceFixedOrder{},
		&model.MarketplaceFixedOrderFill{},
		&model.MarketplacePoolFill{},
		&model.MarketplaceSettlement{},
	))
	model.InitOptionMap()
	setting.MarketplaceEnabled = true
	setting.MarketplaceEnabledVendorTypes = []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAnthropic}
	setting.MarketplaceFeeRate = 0
	setting.MarketplaceMinFixedOrderQuota = 100
	setting.MarketplaceMaxFixedOrderQuota = 100000
	setting.MarketplaceFixedOrderDefaultExpirySeconds = 3600
	setting.MarketplaceMaxSellerMultiplier = 10
	setting.MarketplaceMaxCredentialConcurrency = 5

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		common.OptionMap = originalOptionMap
		setting.MarketplaceEnabled = originalMarketplaceEnabled
		setting.MarketplaceEnabledVendorTypes = originalVendorTypes
		setting.MarketplaceFeeRate = originalFeeRate
		setting.MarketplaceMinFixedOrderQuota = originalMinFixedOrderQuota
		setting.MarketplaceMaxFixedOrderQuota = originalMaxFixedOrderQuota
		setting.MarketplaceFixedOrderDefaultExpirySeconds = originalFixedOrderExpiry
		setting.MarketplaceMaxSellerMultiplier = originalMaxMultiplier
		setting.MarketplaceMaxCredentialConcurrency = originalMaxConcurrency
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatio))
		require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(originalTaskPricing))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(originalCompletionRatio))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(originalCacheRatio))
		require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(originalCreateCacheRatio))
		require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(originalImageRatio))
		require.NoError(t, ratio_setting.UpdateAudioRatioByJSONString(originalAudioRatio))
		require.NoError(t, ratio_setting.UpdateAudioCompletionRatioByJSONString(originalAudioCompletionRatio))
	})

	return db
}

func decodeMarketplaceCredentialResponse(t *testing.T, response tokenAPIResponse) marketplaceCredentialResponse {
	t.Helper()

	var credential marketplaceCredentialResponse
	require.NoError(t, json.Unmarshal(response.Data, &credential))
	return credential
}

func createMarketplaceCredentialViaController(t *testing.T, userID int, apiKey string) marketplaceCredentialResponse {
	t.Helper()

	body := map[string]any{
		"vendor_type":        constant.ChannelTypeOpenAI,
		"api_key":            apiKey,
		"models":             []string{"gpt-4o-mini"},
		"quota_mode":         model.MarketplaceQuotaModeUnlimited,
		"time_mode":          model.MarketplaceTimeModeLimited,
		"time_limit_seconds": 3600,
		"multiplier":         1.25,
		"concurrency_limit":  2,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials", body, userID)
	SellerCreateMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	return decodeMarketplaceCredentialResponse(t, response)
}

func TestSellerFetchMarketplaceCredentialModelsUsesSellerScopedChannelConfig(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer market-key", r.Header.Get("Authorization"))
		assert.Equal(t, "seller-form", r.Header.Get("X-Marketplace-Source"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-market"},{"id":"gpt-market-pro"},{"id":"gpt-market"}]}`))
	}))
	t.Cleanup(upstream.Close)

	body := map[string]any{
		"vendor_type":     constant.ChannelTypeOpenAI,
		"api_key":         "market-key\nsecondary-key",
		"base_url":        upstream.URL,
		"header_override": `{"X-Marketplace-Source":"seller-form"}`,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/fetch-models", body, 10)
	SellerFetchMarketplaceCredentialModels(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var models []string
	require.NoError(t, json.Unmarshal(response.Data, &models))
	assert.Equal(t, []string{"gpt-market", "gpt-market-pro"}, models)
}

func TestSellerListMarketplacePricedModelsUsesChannelModelPricingSource(t *testing.T) {
	withCustomPricingModelSettings(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-priced-tiered": "tiered_expr",
	}, map[string]string{
		"zz-priced-tiered": `tier("base", p * 1 + c * 2)`,
	})
	setupMarketplaceSellerControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/priced-models", nil, 10)
	SellerListMarketplacePricedModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)

	byID := make(map[string]dto.OpenAIModels, len(payload.Data))
	for _, item := range payload.Data {
		byID[item.Id] = item
	}
	require.Contains(t, byID, "zz-priced-fixed")
	require.Contains(t, byID, "zz-priced-ratio")
	require.Contains(t, byID, "zz-priced-task")
	require.Contains(t, byID, "zz-priced-completion")
	require.Contains(t, byID, "zz-priced-cache")
	require.Contains(t, byID, "zz-priced-create-cache")
	require.Contains(t, byID, "zz-priced-image")
	require.Contains(t, byID, "zz-priced-audio")
	require.Contains(t, byID, "zz-priced-audio-completion")
	require.Contains(t, byID, "zz-priced-tiered")
	require.Equal(t, "zz-priced-fixed", byID["zz-priced-fixed"].ModelName)
	require.Equal(t, "price", byID["zz-priced-fixed"].QuotaType)
	require.True(t, byID["zz-priced-fixed"].Configured)
}

func TestSellerCreateMarketplaceCredentialEncryptsKeyAndWritesInitialRows(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	assert.Equal(t, 10, credential.SellerUserID)
	assert.Equal(t, constant.ChannelTypeOpenAI, credential.VendorType)
	assert.Equal(t, "OpenAI", credential.VendorNameSnapshot)
	assert.Equal(t, model.MarketplaceListingStatusListed, credential.ListingStatus)
	assert.Equal(t, model.MarketplaceServiceStatusEnabled, credential.ServiceStatus)
	assert.Equal(t, model.MarketplaceHealthStatusUntested, credential.HealthStatus)
	assert.NotEmpty(t, credential.KeyFingerprint)

	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.NotContains(t, stored.EncryptedAPIKey, "seller-secret-placeholder")
	assert.NotEmpty(t, stored.EncryptedAPIKey)
	assert.Equal(t, model.MarketplaceProbeStatusPending, stored.ProbeStatus)
	assert.Equal(t, 0, stored.ProbeScore)
	assert.Equal(t, 0, stored.ProbeScoreMax)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, db.First(&stats, "credential_id = ?", credential.ID).Error)
}

func TestSellerCreateMarketplaceCredentialPersistsChannelConfig(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	baseURL := "https://proxy.example.com"
	body := map[string]any{
		"vendor_type":         constant.ChannelTypeOpenAI,
		"api_key":             "seller-config-key",
		"models":              []string{"gpt-4o-mini"},
		"quota_mode":          model.MarketplaceQuotaModeUnlimited,
		"time_mode":           model.MarketplaceTimeModeLimited,
		"time_limit_seconds":  7200,
		"multiplier":          1.25,
		"concurrency_limit":   2,
		"base_url":            baseURL,
		"other":               "2024-10-21",
		"test_model":          "gpt-4o-mini",
		"openai_organization": "org-marketplace",
		"model_mapping":       `{"gpt-4o-mini":"upstream-mini"}`,
		"status_code_mapping": `{"429":503}`,
		"setting":             `{"proxy":"http://127.0.0.1:7890"}`,
		"param_override":      `{"temperature":0}`,
		"header_override":     `{"X-Test":"ok"}`,
		"settings":            `{"allow_service_tier":true}`,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials", body, 10)
	SellerCreateMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	credential := decodeMarketplaceCredentialResponse(t, response)

	assert.Equal(t, baseURL, credential.BaseURL)
	assert.Equal(t, "2024-10-21", credential.Other)
	assert.Equal(t, "gpt-4o-mini", credential.TestModel)
	assert.Equal(t, "org-marketplace", credential.OpenAIOrganization)
	assert.Equal(t, model.MarketplaceTimeModeLimited, credential.TimeMode)
	assert.Equal(t, int64(7200), credential.TimeLimitSeconds)
	assert.JSONEq(t, `{"gpt-4o-mini":"upstream-mini"}`, credential.ModelMapping)
	assert.JSONEq(t, `{"429":503}`, credential.StatusCodeMapping)
	assert.JSONEq(t, `{"proxy":"http://127.0.0.1:7890"}`, credential.Setting)
	assert.JSONEq(t, `{"temperature":0}`, credential.ParamOverride)
	assert.JSONEq(t, `{"X-Test":"ok"}`, credential.HeaderOverride)
	assert.JSONEq(t, `{"allow_service_tier":true}`, credential.OtherSettings)

	channel, err := service.BuildSellerMarketplaceChannel(10, credential.ID)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, channel.Id)
	assert.Equal(t, constant.ChannelTypeOpenAI, channel.Type)
	assert.Equal(t, "seller-config-key", channel.Key)
	assert.Equal(t, baseURL, channel.GetBaseURL())
	assert.JSONEq(t, `{"gpt-4o-mini":"upstream-mini"}`, channel.GetModelMapping())

	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.NotContains(t, stored.EncryptedAPIKey, "seller-config-key")
}

func TestSellerCreateMarketplaceCredentialRejectsHostedMarketplaceBackedToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       20,
		Username: "marketplace-backed-token-owner",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "marketplace_backed_token_owner",
	}).Error)
	token := &model.Token{
		Id:                      20,
		UserId:                  20,
		Key:                     "hostedfixedtoken",
		Status:                  common.TokenStatusEnabled,
		ExpiredTime:             -1,
		RemainQuota:             10000,
		Group:                   "default",
		MarketplaceRouteEnabled: model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup}),
	}
	token.SetMarketplaceFixedOrderIDList([]int{123})
	require.NoError(t, db.Create(token).Error)

	body := map[string]any{
		"vendor_type":        constant.ChannelTypeOpenAI,
		"api_key":            "sk-hostedfixedtoken",
		"models":             []string{"gpt-4o-mini"},
		"quota_mode":         model.MarketplaceQuotaModeUnlimited,
		"time_mode":          model.MarketplaceTimeModeLimited,
		"time_limit_seconds": 3600,
		"multiplier":         1.25,
		"concurrency_limit":  2,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials", body, 20)
	SellerCreateMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "marketplace fixed order")
}

func TestSellerCreateMarketplaceCredentialAllowsHostedNormalGroupToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       21,
		Username: "group-token-owner",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "group_token_owner",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:                      21,
		UserId:                  21,
		Key:                     "hostedgrouptoken",
		Status:                  common.TokenStatusEnabled,
		ExpiredTime:             -1,
		RemainQuota:             10000,
		Group:                   "default",
		MarketplaceRouteEnabled: model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup}),
	}).Error)

	credential := createMarketplaceCredentialViaController(t, 21, "sk-hostedgrouptoken")

	assert.Equal(t, 21, credential.SellerUserID)
	assert.NotEmpty(t, credential.KeyFingerprint)
}

func TestSellerCreateMarketplaceCredentialAllowsHostedDefaultRouteToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       23,
		Username: "default-route-token-owner",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "default_route_token_owner",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          23,
		UserId:      23,
		Key:         "hosteddefaultroutetoken",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10000,
		Group:       "default",
	}).Error)

	credential := createMarketplaceCredentialViaController(t, 23, "sk-hosteddefaultroutetoken")

	assert.Equal(t, 23, credential.SellerUserID)
	assert.NotEmpty(t, credential.KeyFingerprint)
}

func TestSellerUpdateMarketplaceCredentialRejectsHostedPoolToken(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Create(&model.User{
		Id:       22,
		Username: "pool-token-owner",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "pool_token_owner",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:                            22,
		UserId:                        22,
		Key:                           "hostedpooltoken",
		Status:                        common.TokenStatusEnabled,
		ExpiredTime:                   -1,
		RemainQuota:                   10000,
		Group:                         "default",
		MarketplaceRouteEnabled:       model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRoutePool}),
		MarketplacePoolFiltersEnabled: true,
		MarketplacePoolFilters: model.NewMarketplacePoolFilters(model.MarketplacePoolFilterValues{
			Model: "gpt-4o-mini",
		}),
	}).Error)

	body := map[string]any{
		"api_key": "sk-hostedpooltoken",
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/marketplace/seller/credentials/1", body, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerUpdateMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "marketplace order pool")
}

func TestSellerListMarketplaceCredentialsIncludesStatsAndFreshCapacity(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-limited-stats")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"quota_mode":       model.MarketplaceQuotaModeLimited,
			"quota_limit":      100000,
			"capacity_status":  model.MarketplaceCapacityStatusAvailable,
			"probe_status":     model.MarketplaceProbeStatusPassed,
			"probe_score":      91,
			"probe_score_max":  96,
			"probe_grade":      "A",
			"probe_checked_at": int64(1710000000),
		}).Error)
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", credential.ID).
		Updates(map[string]any{
			"quota_used":          100000,
			"current_concurrency": 1,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/credentials", nil, 10)
	SellerListMarketplaceCredentials(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var items []marketplaceCredentialResponse
	require.NoError(t, json.Unmarshal(page.Items, &items))
	require.Len(t, items, 1)
	assert.Equal(t, int64(100000), items[0].QuotaUsed)
	assert.Equal(t, 1, items[0].CurrentConcurrency)
	assert.Equal(t, model.MarketplaceCapacityStatusExhausted, items[0].CapacityStatus)
	assert.Equal(t, model.MarketplaceRouteStatusExhausted, items[0].RouteStatus)
	assert.Equal(t, model.MarketplaceProbeStatusPassed, items[0].ProbeStatus)
	assert.Equal(t, 91, items[0].ProbeScore)
	assert.Equal(t, 96, items[0].ProbeScoreMax)
	assert.Equal(t, "A", items[0].ProbeGrade)
	assert.Equal(t, int64(1710000000), items[0].ProbeCheckedAt)
}

func TestSellerListMarketplaceCredentialsReportsFailedHealthAsNotRoutable(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-failed-route-status")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusFailed,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/credentials", nil, 10)
	SellerListMarketplaceCredentials(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(response.Data, &page))
	var items []marketplaceCredentialResponse
	require.NoError(t, json.Unmarshal(page.Items, &items))
	require.Len(t, items, 1)
	assert.Equal(t, model.MarketplaceHealthStatusFailed, items[0].HealthStatus)
	assert.Equal(t, model.MarketplaceCapacityStatusAvailable, items[0].CapacityStatus)
	assert.Equal(t, model.MarketplaceRouteStatusFailed, items[0].RouteStatus)
	assert.Equal(t, model.MarketplaceRouteReasonHealthFailed, items[0].RouteReason)
}

func TestApplySellerMarketplaceCredentialTestResultUpdatesHealth(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	updated, err := service.ApplySellerMarketplaceCredentialTestResult(service.MarketplaceCredentialTestResultInput{
		SellerUserID:   10,
		CredentialID:   credential.ID,
		Success:        true,
		ResponseTimeMS: 123,
		Reason:         "ok",
	})
	require.NoError(t, err)
	assert.Equal(t, model.MarketplaceHealthStatusHealthy, updated.HealthStatus)
	assert.Equal(t, model.MarketplaceCapacityStatusAvailable, updated.CapacityStatus)
	assert.Equal(t, 123, updated.ResponseTime)
	assert.Greater(t, updated.TestTime, int64(0))

	updated, err = service.ApplySellerMarketplaceCredentialTestResult(service.MarketplaceCredentialTestResultInput{
		SellerUserID:   10,
		CredentialID:   credential.ID,
		Success:        false,
		ResponseTimeMS: 456,
		Reason:         "upstream unauthorized: seller-secret-placeholder",
	})
	require.NoError(t, err)
	assert.Equal(t, model.MarketplaceHealthStatusFailed, updated.HealthStatus)
	assert.Equal(t, 456, updated.ResponseTime)
	assert.Greater(t, updated.TestTime, int64(0))
}

func TestSellerCreateMarketplaceCredentialRejectsDisabledVendor(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)

	body := map[string]any{
		"vendor_type":       constant.ChannelTypeMidjourney,
		"api_key":           "seller-secret-placeholder",
		"models":            []string{"mj"},
		"quota_mode":        model.MarketplaceQuotaModeUnlimited,
		"multiplier":        1.25,
		"concurrency_limit": 2,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials", body, 10)
	SellerCreateMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not enabled")
}

func TestSellerMarketplaceCredentialOwnershipIsolation(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/credentials/1", nil, 11)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerGetMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.NotContains(t, recorder.Body.String(), "seller-secret-placeholder")
}

func TestSellerDeleteMarketplaceCredentialRemovesOwnedCredentialAndStats(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/marketplace/seller/credentials/1", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerDeleteMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "seller-secret-placeholder")

	var credentialCount int64
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Count(&credentialCount).Error)
	assert.Equal(t, int64(0), credentialCount)

	var statsCount int64
	require.NoError(t, db.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", credential.ID).
		Count(&statsCount).Error)
	assert.Equal(t, int64(0), statsCount)
}

func TestSellerDeleteMarketplaceCredentialRejectsOtherSeller(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/marketplace/seller/credentials/1", nil, 11)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerDeleteMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.NotContains(t, recorder.Body.String(), "seller-secret-placeholder")

	var credentialCount int64
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Count(&credentialCount).Error)
	assert.Equal(t, int64(1), credentialCount)
}

func TestSellerDeleteMarketplaceCredentialRejectsActiveFixedOrders(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Create(&model.MarketplaceFixedOrder{
		BuyerUserID:    20,
		SellerUserID:   10,
		CredentialID:   credential.ID,
		PurchasedQuota: 1000,
		RemainingQuota: 1000,
		Status:         model.MarketplaceFixedOrderStatusActive,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/marketplace/seller/credentials/1", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerDeleteMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.Contains(t, response.Message, "active fixed orders")

	var credentialCount int64
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Count(&credentialCount).Error)
	assert.Equal(t, int64(1), credentialCount)
}

func TestSellerUpdateMarketplaceCredentialEditsConfigAndReplacesKey(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	body := map[string]any{
		"api_key":            "new-secret-placeholder",
		"models":             []string{"gpt-4o-mini", "gpt-4o"},
		"quota_mode":         model.MarketplaceQuotaModeLimited,
		"quota_limit":        10000,
		"time_mode":          model.MarketplaceTimeModeLimited,
		"time_limit_seconds": 86400,
		"multiplier":         2.5,
		"concurrency_limit":  4,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/marketplace/seller/credentials/1", body, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerUpdateMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	updated := decodeMarketplaceCredentialResponse(t, response)
	assert.Equal(t, model.MarketplaceQuotaModeLimited, updated.QuotaMode)
	assert.Equal(t, int64(10000), updated.QuotaLimit)
	assert.Equal(t, model.MarketplaceTimeModeLimited, updated.TimeMode)
	assert.Equal(t, int64(86400), updated.TimeLimitSeconds)
	assert.Equal(t, 2.5, updated.Multiplier)
	assert.Equal(t, 4, updated.ConcurrencyLimit)
	assert.Contains(t, updated.Models, "gpt-4o")
	assert.NotEqual(t, credential.KeyFingerprint, updated.KeyFingerprint)

	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.NotContains(t, stored.EncryptedAPIKey, "new-secret-placeholder")
}

func TestSellerMarketplaceCredentialLifecycleActionsUpdateStatus(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	actions := []struct {
		name      string
		handler   func(*gin.Context)
		wantField string
		wantValue string
	}{
		{"unlist", SellerUnlistMarketplaceCredential, "listing_status", model.MarketplaceListingStatusUnlisted},
		{"list", SellerListMarketplaceCredential, "listing_status", model.MarketplaceListingStatusListed},
		{"disable", SellerDisableMarketplaceCredential, "service_status", model.MarketplaceServiceStatusDisabled},
		{"enable", SellerEnableMarketplaceCredential, "service_status", model.MarketplaceServiceStatusEnabled},
	}

	for _, action := range actions {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/"+action.name, nil, 10)
		ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
		action.handler(ctx)
		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, "%s: %s", action.name, response.Message)
		assert.Contains(t, recorder.Body.String(), action.wantValue)
	}
}

func TestSellerProbeMarketplaceCredentialQueuesOwnedCredential(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusUnscored,
			"probe_score":      0,
			"probe_score_max":  0,
			"probe_checked_at": 0,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/probe", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerProbeMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	updated := decodeMarketplaceCredentialResponse(t, response)
	assert.Equal(t, model.MarketplaceProbeStatusPending, updated.ProbeStatus)
	assert.Zero(t, updated.ProbeCheckedAt)
}

func TestSellerProbeMarketplaceCredentialAcceptsRequestedModel(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"models":           "gpt-4o-mini,gpt-5.5",
			"probe_status":     model.MarketplaceProbeStatusUnscored,
			"probe_score":      0,
			"probe_score_max":  0,
			"probe_checked_at": 0,
		}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/probe?model=gpt-5.5", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerProbeMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	updated := decodeMarketplaceCredentialResponse(t, response)
	assert.Equal(t, model.MarketplaceProbeStatusPending, updated.ProbeStatus)
	var stored model.MarketplaceCredential
	require.NoError(t, db.First(&stored, credential.ID).Error)
	assert.Equal(t, "gpt-5.5", stored.ProbeModel)
}

func TestSellerProbeMarketplaceCredentialRejectsOtherSeller(t *testing.T) {
	setupMarketplaceSellerControllerTestDB(t)
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/probe", nil, 11)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerProbeMarketplaceCredential(ctx)
	response := decodeAPIResponse(t, recorder)

	require.False(t, response.Success)
	assert.Contains(t, response.Message, "not found")
}

func TestSellerTestMarketplaceCredentialDoesNotExposeSecret(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	testUser := &model.User{
		Id:       1,
		Username: "marketplace_test_root",
		Password: "password123",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "marketplace_test_root",
	}
	testUser.SetSetting(dto.UserSetting{AcceptUnsetRatioModel: true})
	require.NoError(t, db.Create(testUser).Error)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"gpt-4o-mini",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}
		}`))
	}))
	defer upstream.Close()
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Update("base_url", upstream.URL).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/test", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerTestMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NotContains(t, recorder.Body.String(), "seller-secret-placeholder")
	updated := decodeMarketplaceCredentialResponse(t, response)
	assert.Greater(t, updated.ResponseTime, 0)
	assert.Greater(t, updated.TestTime, int64(0))
}

func TestSellerTestMarketplaceCredentialAcceptsOpenAICompatibleV1BaseURL(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	testUser := &model.User{
		Id:       1,
		Username: "marketplace_test_v1_base",
		Password: "password123",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "marketplace_test_v1_base",
	}
	testUser.SetSetting(dto.UserSetting{AcceptUnsetRatioModel: true})
	require.NoError(t, db.Create(testUser).Error)
	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"gpt-4o-mini",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}
		}`))
	}))
	defer upstream.Close()
	credential := createMarketplaceCredentialViaController(t, 10, "seller-secret-placeholder")
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Update("base_url", upstream.URL+"/v1").Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/credentials/1/test", nil, 10)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", credential.ID)}}
	SellerTestMarketplaceCredential(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.Equal(t, "/v1/chat/completions", upstreamPath)
}

func TestSellerMarketplaceIncomeReleaseAndSettlementList(t *testing.T) {
	db := setupMarketplaceSellerControllerTestDB(t)
	sellerID := 10
	buyerID := 20
	seedMarketplaceUser(t, db, sellerID, 0)
	seedMarketplaceUser(t, db, buyerID, 10000)
	credential := createHealthyMarketplaceCredential(t, db, sellerID, "seller-income-placeholder")
	now := common.GetTimestamp()
	available := seedControllerMarketplaceSettlement(t, db, buyerID, sellerID, credential.ID, "seller-available", 500, model.MarketplaceSettlementStatusAvailable, now-1)
	seedControllerMarketplaceSettlement(t, db, buyerID, sellerID, credential.ID, "seller-due", 760, model.MarketplaceSettlementStatusPending, now-1)
	seedControllerMarketplaceSettlement(t, db, buyerID, sellerID, credential.ID, "seller-future", 300, model.MarketplaceSettlementStatusPending, now+3600)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/income", nil, sellerID)
	SellerGetMarketplaceIncome(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var summary service.MarketplaceIncomeSummary
	require.NoError(t, json.Unmarshal(response.Data, &summary))
	assert.Equal(t, int64(1060), summary.PendingIncome)
	assert.Equal(t, int64(500), summary.AvailableIncome)

	ctx, releaseRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/marketplace/seller/settlements/release", nil, sellerID)
	SellerReleaseMarketplaceSettlements(ctx)
	releaseResponse := decodeAPIResponse(t, releaseRecorder)
	require.True(t, releaseResponse.Success, releaseResponse.Message)
	var releaseResult service.MarketplaceSettlementReleaseResult
	require.NoError(t, json.Unmarshal(releaseResponse.Data, &releaseResult))
	assert.Equal(t, 1, releaseResult.ReleasedCount)
	assert.Equal(t, int64(760), releaseResult.ReleasedIncome)

	var seller model.User
	require.NoError(t, db.First(&seller, sellerID).Error)
	assert.Equal(t, 760, seller.Quota)

	ctx, listRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/marketplace/seller/settlements?status=available", nil, sellerID)
	SellerListMarketplaceSettlements(ctx)
	listResponse := decodeAPIResponse(t, listRecorder)
	require.True(t, listResponse.Success, listResponse.Message)
	var page marketplacePageResponse
	require.NoError(t, json.Unmarshal(listResponse.Data, &page))
	var settlements []model.MarketplaceSettlement
	require.NoError(t, json.Unmarshal(page.Items, &settlements))
	require.Len(t, settlements, 2)
	assert.Contains(t, []int{settlements[0].ID, settlements[1].ID}, available.ID)
}

func seedControllerMarketplaceSettlement(t *testing.T, db *gorm.DB, buyerID int, sellerID int, credentialID int, requestID string, sellerIncome int64, status string, availableAt int64) model.MarketplaceSettlement {
	t.Helper()

	settlement := model.MarketplaceSettlement{
		RequestID:               requestID,
		BuyerUserID:             buyerID,
		SellerUserID:            sellerID,
		CredentialID:            credentialID,
		SourceType:              "pool_fill",
		SourceID:                requestID,
		BuyerCharge:             sellerIncome,
		SellerIncome:            sellerIncome,
		OfficialCost:            sellerIncome,
		MultiplierSnapshot:      1,
		PlatformFeeRateSnapshot: 0,
		Status:                  status,
		AvailableAt:             availableAt,
	}
	require.NoError(t, db.Create(&settlement).Error)
	return settlement
}
