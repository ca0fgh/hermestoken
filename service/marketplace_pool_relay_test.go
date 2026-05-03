package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type marketplacePoolRelayFixture struct {
	DB           *gorm.DB
	BuyerUserID  int
	SellerUserID int
	Token        *model.Token
	Credential   *model.MarketplaceCredential
}

func newMarketplacePoolRelayFixture(t *testing.T) marketplacePoolRelayFixture {
	t.Helper()

	db := setupMarketplaceFixedOrderRelayTestDB(t)
	buyerID := 20
	sellerID := 10
	require.NoError(t, db.Create(&model.User{
		Id:       buyerID,
		Username: "marketplace_pool_buyer",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "pool_buyer",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       sellerID,
		Username: "marketplace_pool_seller",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    0,
		AffCode:  "pool_seller",
	}).Error)
	token := &model.Token{
		Id:          1,
		UserId:      buyerID,
		Key:         "pool-token",
		Name:        "pool token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 10000,
	}
	require.NoError(t, db.Create(token).Error)

	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     sellerID,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-pool-key",
		Models:           []string{"gpt-4o-mini"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1.25,
		ConcurrencyLimit: 2,
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
		}).Error)
	require.NoError(t, db.First(credential, credential.ID).Error)

	return marketplacePoolRelayFixture{
		DB:           db,
		BuyerUserID:  buyerID,
		SellerUserID: sellerID,
		Token:        token,
		Credential:   credential,
	}
}

func TestPrepareMarketplacePoolRelaySelectsEligibleCredentialAndIncrementsConcurrency(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	busyCredential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     11,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-pool-busy-key",
		Models:           []string{"gpt-4o-mini"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1.0,
		ConcurrencyLimit: 1,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", busyCredential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
		}).Error)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", busyCredential.ID).
		Update("current_concurrency", 1).Error)

	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-prepare",
	})
	require.NoError(t, err)
	assert.Equal(t, fixture.Credential.ID, preparation.Credential.ID)
	assert.Equal(t, "marketplace-pool-key", preparation.APIKey)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 1, stats.CurrentConcurrency)
}

func TestPrepareMarketplacePoolRelayHonorsBuyerFilters(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)

	_, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID:   fixture.BuyerUserID,
		Model:         "gpt-4o-mini",
		MaxMultiplier: 1.0,
		RequestID:     "pool-filtered-out",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible")

	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID:   fixture.BuyerUserID,
		Model:         "gpt-4o-mini",
		MinMultiplier: 1.2,
		MaxMultiplier: 1.3,
		QuotaMode:     model.MarketplaceQuotaModeUnlimited,
		RequestID:     "pool-filtered-in",
	})
	require.NoError(t, err)
	assert.Equal(t, fixture.Credential.ID, preparation.Credential.ID)
}

func TestMarketplacePoolRelayBillingSessionSettleChargesWalletAndWritesSettlement(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-settle",
	})
	require.NoError(t, err)

	normalBilling := newMarketplacePoolNormalBilling(t, fixture, 1000)
	poolBilling := NewMarketplacePoolBillingSession(normalBilling, preparation.Session)
	require.NoError(t, poolBilling.Settle(800))

	var buyer model.User
	require.NoError(t, fixture.DB.First(&buyer, fixture.BuyerUserID).Error)
	assert.Equal(t, 9200, buyer.Quota)

	var token model.Token
	require.NoError(t, fixture.DB.First(&token, fixture.Token.Id).Error)
	assert.Equal(t, 9200, token.RemainQuota)

	var fill model.MarketplacePoolFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "pool-settle").Error)
	assert.Equal(t, fixture.BuyerUserID, fill.BuyerUserID)
	assert.Equal(t, fixture.SellerUserID, fill.SellerUserID)
	assert.Equal(t, fixture.Credential.ID, fill.CredentialID)
	assert.Equal(t, "gpt-4o-mini", fill.Model)
	assert.Equal(t, int64(640), fill.OfficialCost)
	assert.Equal(t, int64(800), fill.BuyerCharge)
	assert.Equal(t, model.MarketplaceFillStatusSucceeded, fill.Status)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "pool-settle").Error)
	assert.Equal(t, "pool_fill", settlement.SourceType)
	assert.Equal(t, fmt.Sprintf("%d", fill.ID), settlement.SourceID)
	assert.Equal(t, int64(800), settlement.BuyerCharge)
	assert.Equal(t, int64(0), settlement.PlatformFee)
	assert.Equal(t, int64(800), settlement.SellerIncome)
	assert.Equal(t, int64(640), settlement.OfficialCost)
	assert.Equal(t, model.MarketplaceSettlementStatusAvailable, settlement.Status)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 800, seller.Quota)

	var sellerLog model.Log
	require.NoError(t, fixture.DB.First(&sellerLog, "user_id = ? AND type = ?", fixture.SellerUserID, model.LogTypeSystem).Error)
	assert.Contains(t, sellerLog.Content, "市场订单池收益实时结算")
	assert.Contains(t, sellerLog.Content, "＄0.001600")

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 0, stats.CurrentConcurrency)
	assert.Equal(t, int64(1), stats.PoolRequestCount)
	assert.Equal(t, int64(1), stats.TotalRequestCount)
	assert.Equal(t, int64(640), stats.QuotaUsed)
	assert.Equal(t, int64(1), stats.SuccessCount)
}

func TestMarketplacePoolRelayBillingSessionSettleChargesWalletWithTransactionFee(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	setting.MarketplaceFeeRate = 0.05

	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-settle-fee",
	})
	require.NoError(t, err)

	normalBilling := newMarketplacePoolNormalBilling(t, fixture, 1050)
	poolBilling := NewMarketplacePoolBillingSession(normalBilling, preparation.Session)
	require.NoError(t, poolBilling.Settle(800))

	var buyer model.User
	require.NoError(t, fixture.DB.First(&buyer, fixture.BuyerUserID).Error)
	assert.Equal(t, 9160, buyer.Quota)

	var token model.Token
	require.NoError(t, fixture.DB.First(&token, fixture.Token.Id).Error)
	assert.Equal(t, 9160, token.RemainQuota)

	var fill model.MarketplacePoolFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "pool-settle-fee").Error)
	assert.Equal(t, int64(840), fill.BuyerCharge)
	assert.Equal(t, int64(640), fill.OfficialCost)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "pool-settle-fee").Error)
	assert.Equal(t, int64(840), settlement.BuyerCharge)
	assert.Equal(t, int64(40), settlement.PlatformFee)
	assert.Equal(t, 0.05, settlement.PlatformFeeRateSnapshot)
	assert.Equal(t, int64(800), settlement.SellerIncome)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 800, seller.Quota)
}

func TestMarketplacePoolRelayBuyerChargeForQuotaIncludesTransactionFee(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	setting.MarketplaceFeeRate = 0.05

	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-charge-preview-fee",
	})
	require.NoError(t, err)

	assert.Equal(t, 1050, preparation.Session.BuyerChargeForQuota(1000))
	assert.Equal(t, 840, preparation.Session.BuyerChargeForQuota(800))
}

func TestMarketplacePoolRelayBillingSessionRefundRestoresWalletAndReleasesConcurrency(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	preparation, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-refund",
	})
	require.NoError(t, err)

	normalBilling := newMarketplacePoolNormalBilling(t, fixture, 1000)
	poolBilling := NewMarketplacePoolBillingSession(normalBilling, preparation.Session)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	poolBilling.Refund(ctx)

	var buyer model.User
	var token model.Token
	require.Eventually(t, func() bool {
		require.NoError(t, fixture.DB.First(&buyer, fixture.BuyerUserID).Error)
		require.NoError(t, fixture.DB.First(&token, fixture.Token.Id).Error)
		return buyer.Quota == 10000 && token.RemainQuota == 10000
	}, time.Second, 10*time.Millisecond)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 0, stats.CurrentConcurrency)

	var fillCount int64
	require.NoError(t, fixture.DB.Model(&model.MarketplacePoolFill{}).Count(&fillCount).Error)
	assert.Equal(t, int64(0), fillCount)
}

func TestPrepareMarketplacePoolRelayRejectsWhenNoEligibleCredential(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", fixture.Credential.ID).
		Update("current_concurrency", 2).Error)

	_, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-no-candidate",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible")
}

func TestPrepareMarketplacePoolRelayRejectsOwnCredential(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", fixture.Credential.ID).
		Update("seller_user_id", fixture.BuyerUserID).Error)

	_, err := PrepareMarketplacePoolRelay(MarketplacePoolRelayInput{
		BuyerUserID: fixture.BuyerUserID,
		Model:       "gpt-4o-mini",
		RequestID:   "pool-own-candidate",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible")
}

func newMarketplacePoolNormalBilling(t *testing.T, fixture marketplacePoolRelayFixture, preConsumedQuota int) relaycommon.BillingSettler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("token_quota", fixture.Token.RemainQuota)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "pool-settle",
		UserId:          fixture.BuyerUserID,
		UserQuota:       10000,
		TokenId:         fixture.Token.Id,
		TokenKey:        fixture.Token.Key,
		TokenGroup:      "default",
		UsingGroup:      "default",
		UserGroup:       "default",
		OriginModelName: "gpt-4o-mini",
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}
	billing, apiErr := NewBillingSession(ctx, relayInfo, preConsumedQuota)
	require.Nil(t, apiErr)
	return billing
}
