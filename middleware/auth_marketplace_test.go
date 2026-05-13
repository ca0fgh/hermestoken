package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMarketplaceTokenAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	originalRedis := common.RedisEnabled
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMarketplaceEnabled := setting.MarketplaceEnabled
	originalMarketplaceEnabledVendorTypes := append([]int(nil), setting.MarketplaceEnabledVendorTypes...)

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.MarketplaceCredential{},
		&model.MarketplaceCredentialStats{},
		&model.MarketplaceFixedOrder{},
	))

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"standard":"标准价格"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"standard":1}`))
	require.NoError(t, types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, `{}`))
	setting.MarketplaceEnabled = true
	setting.MarketplaceEnabledVendorTypes = []int{constant.ChannelTypeOpenAI}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
		common.RedisEnabled = originalRedis
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecialGroups))
		setting.MarketplaceEnabled = originalMarketplaceEnabled
		setting.MarketplaceEnabledVendorTypes = originalMarketplaceEnabledVendorTypes
	})

	return db
}

func seedMarketplaceBoundToken(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "marketplace-token-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)

	token := &model.Token{
		Id:          1,
		UserId:      1,
		Key:         "marketplacetoken",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10000,
	}
	token.SetMarketplaceFixedOrderIDList([]int{1, 2})
	require.NoError(t, db.Create(token).Error)
}

func seedDefaultGroupChannel(t *testing.T, db *gorm.DB, channelID int, modelName string) {
	t.Helper()

	seedGroupChannel(t, db, channelID, "default", modelName)
}

func seedGroupChannel(t *testing.T, db *gorm.DB, channelID int, groupName string, modelName string) {
	t.Helper()

	priority := int64(0)
	require.NoError(t, db.Create(&model.Channel{
		Id:       channelID,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "normal-group-channel-key",
		Status:   common.ChannelStatusEnabled,
		Name:     groupName + " group channel",
		Models:   modelName,
		Group:    groupName,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     groupName,
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
		Weight:    0,
	}).Error)
}

func seedStandardGroupTokenWithFixedOrder(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&model.User{
		Id:       4,
		Username: "standard-bound-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "standard_bound_buyer",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       5,
		Username: "marketplace-seller",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "marketplace_seller",
	}).Error)
	token := &model.Token{
		Id:          4,
		UserId:      4,
		Key:         "standardboundtoken",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10000,
		Group:       "standard",
	}
	token.SetMarketplaceFixedOrderIDList([]int{11})
	require.NoError(t, db.Create(token).Error)
	require.NoError(t, db.Create(&model.MarketplaceCredential{
		ID:                 11,
		SellerUserID:       5,
		VendorType:         constant.ChannelTypeOpenAI,
		VendorNameSnapshot: "OpenAI",
		EncryptedAPIKey:    "encrypted",
		KeyFingerprint:     "fingerprint",
		Models:             "gpt-5.5",
		QuotaMode:          model.MarketplaceQuotaModeUnlimited,
		TimeMode:           model.MarketplaceTimeModeUnlimited,
		Multiplier:         1,
		ConcurrencyLimit:   1,
		ListingStatus:      model.MarketplaceListingStatusListed,
		ServiceStatus:      model.MarketplaceServiceStatusEnabled,
		HealthStatus:       model.MarketplaceHealthStatusHealthy,
		CapacityStatus:     model.MarketplaceCapacityStatusAvailable,
		RiskStatus:         model.MarketplaceRiskStatusNormal,
	}).Error)
	require.NoError(t, db.Create(&model.MarketplaceFixedOrder{
		ID:                      11,
		BuyerUserID:             4,
		SellerUserID:            5,
		CredentialID:            11,
		PurchasedQuota:          10000,
		RemainingQuota:          10000,
		MultiplierSnapshot:      1,
		PlatformFeeRateSnapshot: 0,
		ExpiresAt:               common.GetTimestamp() + 3600,
		Status:                  model.MarketplaceFixedOrderStatusActive,
	}).Error)
}

func TestTokenAuthAllowsMarketplaceModelListForBlankGroupToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)

	router := gin.New()
	router.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceModelList))
		value, ok := c.Get("token_marketplace_fixed_order_ids")
		require.True(t, ok)
		require.Equal(t, []int{1, 2}, value)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTokenAuthAllowsMarketplaceBoundTokenWithoutSelectableGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		value, ok := c.Get("token_marketplace_fixed_order_ids")
		require.True(t, ok)
		require.Equal(t, []int{1, 2}, value)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTokenAuthAllowsMarketplaceBoundTokenOnRootResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)

	router := gin.New()
	router.POST("/responses", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		value, ok := c.Get("token_marketplace_fixed_order_ids")
		require.True(t, ok)
		require.Equal(t, []int{1, 2}, value)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/responses", nil)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTokenAuthAllowsMarketplacePoolTokenWithoutSelectableGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       2,
		Username: "marketplace-pool-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          2,
		UserId:      2,
		Key:         "marketplacepooltoken",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10000,
	}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-marketplacepooltoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDistributeSkipsNormalChannelSelectionForRootResponsesUnifiedMarketplaceRelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)

	router := gin.New()
	router.POST("/responses", TokenAuth(), Distribute(), func(c *gin.Context) {
		require.Equal(t, "gpt-5.5", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/responses",
		bytes.NewBufferString(`{"model":"gpt-5.5","input":"hello"}`),
	)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTokenAuthRejectsBlankGroupWhenMarketplaceRoutesDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       6,
		Username: "disabled-marketplace-routes-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:                      6,
		UserId:                  6,
		Key:                     "disabledmarketplaceroutestoken",
		Status:                  common.TokenStatusEnabled,
		ExpiredTime:             -1,
		RemainQuota:             10000,
		MarketplaceRouteEnabled: model.NewMarketplaceRouteEnabled(nil),
	}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		t.Fatal("request should be rejected before handler")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-disabledmarketplaceroutestoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "当前令牌未配置可用分组")
}

func TestTokenAuthRejectsBlankDefaultGroupWhenOnlyMarketplaceGroupRouteEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       8,
		Username: "group-only-marketplace-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:                      8,
		UserId:                  8,
		Key:                     "grouponlymarketplacetoken",
		Status:                  common.TokenStatusEnabled,
		ExpiredTime:             -1,
		RemainQuota:             10000,
		MarketplaceRouteEnabled: model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup}),
	}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		t.Fatal("request should be rejected before handler")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-grouponlymarketplacetoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "当前令牌未配置可用分组")
}

func TestTokenAuthDoesNotMarkNormalGroupRelayAsUnifiedMarketplace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       3,
		Username: "standard-group-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          3,
		UserId:      3,
		Key:         "standardtoken",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10000,
		Group:       "standard",
	}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, "standard", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.False(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardtoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDistributeSkipsNormalChannelSelectionForUnifiedMarketplaceRelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), Distribute(), func(c *gin.Context) {
		require.Equal(t, "gpt-5.5", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`),
	)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDistributeDoesNotUseDefaultGroupForBlankGroupMarketplaceTokenWhenModelMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedMarketplaceBoundToken(t, db)
	seedDefaultGroupChannel(t, db, 7, "gpt-5.4")

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), Distribute(), func(c *gin.Context) {
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay))
		require.Equal(t, "", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.Equal(t, 0, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
		require.Equal(t, "gpt-5.4", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`),
	)
	request.Header.Set("Authorization", "Bearer sk-marketplacetoken")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDistributeAllowsPlaygroundSubscriptionUpgradeGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"standard":1,"cc-opus-福利渠道":1}`))
	require.NoError(t, db.Create(&model.User{
		Id:       9,
		Username: "playground-subscription-buyer",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId:        9,
		PlanId:        0,
		AmountTotal:   10000,
		AmountUsed:    0,
		StartTime:     common.GetTimestamp() - 60,
		EndTime:       common.GetTimestamp() + 3600,
		Status:        "active",
		Source:        "order",
		UpgradeGroup:  "cc-opus-福利渠道",
		PrevUserGroup: "default",
	}).Error)
	seedGroupChannel(t, db, 9, "cc-opus-福利渠道", "claude-opus-4-7")

	router := gin.New()
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 9)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, Distribute(), func(c *gin.Context) {
		require.Equal(t, "cc-opus-福利渠道", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.Equal(t, 9, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/pg/chat/completions",
		bytes.NewBufferString(`{"model":"claude-opus-4-7","group":"cc-opus-福利渠道"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDistributeRejectsPlaygroundBlankDefaultFallbackWhenDefaultIsNotUsable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       10,
		Username: "playground-default-fallback-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
	}).Error)
	seedDefaultGroupChannel(t, db, 10, "gpt-5.4")

	router := gin.New()
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 10)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, Distribute(), func(c *gin.Context) {
		t.Fatal("request should be rejected before handler")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/pg/chat/completions",
		bytes.NewBufferString(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "当前用户未配置可用分组")
}

func TestDistributePrefersBoundFixedOrderForNormalGroupTokenWhenModelMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), Distribute(), func(c *gin.Context) {
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay))
		require.Equal(t, "gpt-5.5", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		require.Equal(t, "standard", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`),
	)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnifiedMarketplaceRouteOrderCanPreferNormalGroupBeforeFixedOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)
	require.NoError(t, db.Model(&model.Token{}).
		Where("key = ?", "standardboundtoken").
		Update("marketplace_route_order", model.NewMarketplaceRouteOrder([]string{
			model.MarketplaceRouteGroup,
			model.MarketplaceRouteFixedOrder,
			model.MarketplaceRoutePool,
		})).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, []string{
			model.MarketplaceRouteGroup,
			model.MarketplaceRouteFixedOrder,
			model.MarketplaceRoutePool,
		}, common.GetContextKeyStringSlice(c, constant.ContextKeyMarketplaceRouteOrder))
		require.False(t, shouldUseMarketplaceUnifiedRelay(c, "gpt-5.5"))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnifiedMarketplaceRouteEnabledCanDisableFixedOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)
	require.NoError(t, db.Model(&model.Token{}).
		Where("key = ?", "standardboundtoken").
		Updates(map[string]any{
			"marketplace_route_order":   model.NewMarketplaceRouteOrder([]string{model.MarketplaceRouteFixedOrder, model.MarketplaceRouteGroup, model.MarketplaceRoutePool}),
			"marketplace_route_enabled": model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool}),
		}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.Equal(t, []string{
			model.MarketplaceRouteGroup,
			model.MarketplaceRoutePool,
		}, common.GetContextKeyStringSlice(c, constant.ContextKeyMarketplaceRouteEnabled))
		require.False(t, shouldUseMarketplaceUnifiedRelay(c, "gpt-5.5"))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnifiedMarketplaceFixedOrderHeaderStillUsesUnifiedRelayWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)
	require.NoError(t, db.Model(&model.Token{}).
		Where("key = ?", "standardboundtoken").
		Updates(map[string]any{
			"marketplace_route_order":   model.NewMarketplaceRouteOrder([]string{model.MarketplaceRouteFixedOrder, model.MarketplaceRouteGroup, model.MarketplaceRoutePool}),
			"marketplace_route_enabled": model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool}),
		}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.True(t, shouldUseMarketplaceUnifiedRelay(c, "gpt-5.5"))
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	request.Header.Set("X-Marketplace-Fixed-Order-Id", "11")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnifiedMarketplaceRouteOrderFallsBackAfterNormalGroupFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)
	require.NoError(t, db.Model(&model.Token{}).
		Where("key = ?", "standardboundtoken").
		Update("marketplace_route_order", model.NewMarketplaceRouteOrder([]string{
			model.MarketplaceRouteGroup,
			model.MarketplaceRoutePool,
			model.MarketplaceRouteFixedOrder,
		})).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.True(t, shouldFallbackToMarketplaceUnifiedRelayAfterGroup(c, "gpt-5.5"))
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay))
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyMarketplaceSkipGroup))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestUnifiedMarketplaceRouteEnabledCanDisablePoolFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMarketplaceTokenAuthTestDB(t)
	seedStandardGroupTokenWithFixedOrder(t, db)
	require.NoError(t, db.Model(&model.Token{}).
		Where("key = ?", "standardboundtoken").
		Updates(map[string]any{
			"marketplace_route_order":   model.NewMarketplaceRouteOrder([]string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool, model.MarketplaceRouteFixedOrder}),
			"marketplace_route_enabled": model.NewMarketplaceRouteEnabled([]string{model.MarketplaceRouteGroup}),
		}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		require.False(t, shouldFallbackToMarketplaceUnifiedRelayAfterGroup(c, "gpt-5.5"))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-standardboundtoken")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
