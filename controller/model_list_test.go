package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting/config"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func withCustomPricingModelSettings(t *testing.T) {
	t.Helper()

	original := map[string]string{
		"ModelPrice":           ratio_setting.ModelPrice2JSONString(),
		"ModelRatio":           ratio_setting.ModelRatio2JSONString(),
		"TaskModelPricing":     ratio_setting.TaskModelPricing2JSONString(),
		"CompletionRatio":      ratio_setting.CompletionRatio2JSONString(),
		"CacheRatio":           ratio_setting.CacheRatio2JSONString(),
		"CreateCacheRatio":     ratio_setting.CreateCacheRatio2JSONString(),
		"ImageRatio":           ratio_setting.ImageRatio2JSONString(),
		"AudioRatio":           ratio_setting.AudioRatio2JSONString(),
		"AudioCompletionRatio": ratio_setting.AudioCompletionRatio2JSONString(),
	}
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(original["ModelPrice"]))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(original["ModelRatio"]))
		require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(original["TaskModelPricing"]))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(original["CompletionRatio"]))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(original["CacheRatio"]))
		require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(original["CreateCacheRatio"]))
		require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(original["ImageRatio"]))
		require.NoError(t, ratio_setting.UpdateAudioRatioByJSONString(original["AudioRatio"]))
		require.NoError(t, ratio_setting.UpdateAudioCompletionRatioByJSONString(original["AudioCompletionRatio"]))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"zz-priced-fixed":1}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"zz-priced-ratio":2}`))
	require.NoError(t, ratio_setting.UpdateTaskModelPricingByJSONString(`{"zz-priced-task":{"per_request":0.2}}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"zz-priced-completion":3}`))
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"zz-priced-cache":0.5}`))
	require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(`{"zz-priced-create-cache":1.25}`))
	require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(`{"zz-priced-image":1.1}`))
	require.NoError(t, ratio_setting.UpdateAudioRatioByJSONString(`{"zz-priced-audio":1.2}`))
	require.NoError(t, ratio_setting.UpdateAudioCompletionRatioByJSONString(`{"zz-priced-audio-completion":1.3}`))
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)

	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

func TestChannelListPricedModelsUsesModelPricingSettings(t *testing.T) {
	withCustomPricingModelSettings(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-priced-tiered": "tiered_expr",
	}, map[string]string{
		"zz-priced-tiered": `tier("base", p * 1 + c * 2)`,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/models_priced", nil)

	ChannelListPricedModels(ctx)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-priced-fixed")
	require.Contains(t, ids, "zz-priced-ratio")
	require.Contains(t, ids, "zz-priced-task")
	require.Contains(t, ids, "zz-priced-completion")
	require.Contains(t, ids, "zz-priced-cache")
	require.Contains(t, ids, "zz-priced-create-cache")
	require.Contains(t, ids, "zz-priced-image")
	require.Contains(t, ids, "zz-priced-audio")
	require.Contains(t, ids, "zz-priced-audio-completion")
	require.Contains(t, ids, "zz-priced-tiered")
}

func TestChannelListPricedModelsIncludesPricingFields(t *testing.T) {
	withCustomPricingModelSettings(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/models_priced", nil)

	ChannelListPricedModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var pricedFixed dto.OpenAIModels
	for _, item := range payload.Data {
		if item.Id == "zz-priced-fixed" {
			pricedFixed = item
			break
		}
	}
	require.Equal(t, "zz-priced-fixed", pricedFixed.Id)
	require.Equal(t, "zz-priced-fixed", pricedFixed.ModelName)
	require.Equal(t, "price", pricedFixed.QuotaType)
	require.Equal(t, 1.0, pricedFixed.ModelPrice)
	require.True(t, pricedFixed.Configured)
}

func TestChannelListPricedModelsIgnoresChannelTypeFilter(t *testing.T) {
	originalRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
	})

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{
		"gpt-4o": 2,
		"claude-sonnet-4-20250514": 2
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/models_priced?type=1", nil)

	ChannelListPricedModels(ctx)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "gpt-4o")
	require.Contains(t, ids, "claude-sonnet-4-20250514")
}

func TestChannelListPricedModelsIncludesPricingCatalogModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	seedPricingAbility(t, db, "default", "zz-pricing-catalog-only")
	model.RefreshPricing()
	t.Cleanup(model.InvalidatePricingCache)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/models_priced", nil)

	ChannelListPricedModels(ctx)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-pricing-catalog-only")
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}

func TestListModelsReturnsMarketplaceTokenModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	t.Setenv("MARKETPLACE_CREDENTIAL_SECRET", "0123456789abcdef0123456789abcdef")
	require.NoError(t, db.AutoMigrate(
		&model.Token{},
		&model.Log{},
		&model.Option{},
		&model.MarketplaceCredential{},
		&model.MarketplaceCredentialStats{},
		&model.MarketplaceFixedOrder{},
	))

	require.NoError(t, db.Create(&model.User{
		Id:       2001,
		Username: "marketplace-list-buyer",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Quota:    10000,
		AffCode:  "marketplace_list_buyer",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       2002,
		Username: "marketplace-list-seller",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		AffCode:  "marketplace_list_seller",
	}).Error)
	priority := int64(0)
	require.NoError(t, db.Create(&model.Channel{
		Id:       4001,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "normal-group-list-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "normal group list channel",
		Models:   "zz-normal-group",
		Group:    "default",
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "zz-normal-group",
		ChannelId: 4001,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	fixedCredential, err := service.CreateSellerMarketplaceCredential(service.MarketplaceCredentialCreateInput{
		SellerUserID:     2002,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-fixed-key",
		Models:           []string{"zz-marketplace-fixed", "zz-marketplace-shared"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1,
		ConcurrencyLimit: common.GetPointer(2),
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", fixedCredential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
		}).Error)
	order, err := service.CreateMarketplaceFixedOrder(service.MarketplaceFixedOrderCreateInput{
		BuyerUserID:    2001,
		CredentialID:   fixedCredential.ID,
		PurchasedQuota: 1000,
	})
	require.NoError(t, err)

	poolCredential, err := service.CreateSellerMarketplaceCredential(service.MarketplaceCredentialCreateInput{
		SellerUserID:     2002,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-pool-key",
		Models:           []string{"zz-marketplace-pool", "zz-marketplace-shared"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1,
		ConcurrencyLimit: common.GetPointer(2),
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", poolCredential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
		}).Error)

	token := &model.Token{
		Id:          3001,
		UserId:      2001,
		Key:         "marketplace-list-token",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 1000,
	}
	token.SetMarketplaceFixedOrderIDList([]int{order.ID})
	require.NoError(t, db.Create(token).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 2001)
	ctx.Set("token_id", 3001)
	common.SetContextKey(ctx, constant.ContextKeyMarketplaceModelList, true)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-normal-group")
	require.Contains(t, ids, "zz-marketplace-fixed")
	require.Contains(t, ids, "zz-marketplace-pool")
	require.Contains(t, ids, "zz-marketplace-shared")
}
