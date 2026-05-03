package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type marketplaceFixedOrderRelayFixture struct {
	DB           *gorm.DB
	BuyerUserID  int
	SellerUserID int
	Credential   *model.MarketplaceCredential
	Order        *model.MarketplaceFixedOrder
}

func setupMarketplaceFixedOrderRelayTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	originalRedis := common.RedisEnabled
	originalBatchUpdate := common.BatchUpdateEnabled
	originalMarketplaceEnabled := setting.MarketplaceEnabled
	originalVendorTypes := append([]int(nil), setting.MarketplaceEnabledVendorTypes...)
	originalFeeRate := setting.MarketplaceFeeRate
	originalSellerIncomeHold := setting.MarketplaceSellerIncomeHoldSeconds
	originalMinFixedOrderQuota := setting.MarketplaceMinFixedOrderQuota
	originalMaxFixedOrderQuota := setting.MarketplaceMaxFixedOrderQuota
	originalFixedOrderExpiry := setting.MarketplaceFixedOrderDefaultExpirySeconds
	originalMaxMultiplier := setting.MarketplaceMaxSellerMultiplier
	originalMaxConcurrency := setting.MarketplaceMaxCredentialConcurrency

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
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
	setting.MarketplaceSellerIncomeHoldSeconds = 3600
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
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.OptionMap = originalOptionMap
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
		common.RedisEnabled = originalRedis
		common.BatchUpdateEnabled = originalBatchUpdate
		setting.MarketplaceEnabled = originalMarketplaceEnabled
		setting.MarketplaceEnabledVendorTypes = originalVendorTypes
		setting.MarketplaceFeeRate = originalFeeRate
		setting.MarketplaceSellerIncomeHoldSeconds = originalSellerIncomeHold
		setting.MarketplaceMinFixedOrderQuota = originalMinFixedOrderQuota
		setting.MarketplaceMaxFixedOrderQuota = originalMaxFixedOrderQuota
		setting.MarketplaceFixedOrderDefaultExpirySeconds = originalFixedOrderExpiry
		setting.MarketplaceMaxSellerMultiplier = originalMaxMultiplier
		setting.MarketplaceMaxCredentialConcurrency = originalMaxConcurrency
	})

	return db
}

func newMarketplaceFixedOrderRelayFixture(t *testing.T, purchasedQuota int64) marketplaceFixedOrderRelayFixture {
	return newMarketplaceFixedOrderRelayFixtureWithFee(t, purchasedQuota, 0)
}

func newMarketplaceFixedOrderRelayFixtureWithFee(t *testing.T, purchasedQuota int64, feeRate float64) marketplaceFixedOrderRelayFixture {
	t.Helper()

	db := setupMarketplaceFixedOrderRelayTestDB(t)
	setting.MarketplaceFeeRate = feeRate
	buyerQuota := 10000
	if chargedQuota, err := marketplaceBuyerChargeQuotaWithFee(purchasedQuota, feeRate); err == nil {
		if setting.MarketplaceMaxFixedOrderQuota > 0 && chargedQuota > int64(setting.MarketplaceMaxFixedOrderQuota) {
			setting.MarketplaceMaxFixedOrderQuota = int(chargedQuota)
		}
		if chargedQuota > int64(buyerQuota) {
			buyerQuota = int(chargedQuota) + 10000
		}
	}
	buyerID := 20
	sellerID := 10
	require.NoError(t, db.Create(&model.User{
		Id:       buyerID,
		Username: "marketplace_relay_buyer",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    buyerQuota,
		AffCode:  "relay_buyer",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       sellerID,
		Username: "marketplace_relay_seller",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    0,
		AffCode:  "relay_seller",
	}).Error)

	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     sellerID,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-service-key",
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

	order, err := CreateMarketplaceFixedOrder(MarketplaceFixedOrderCreateInput{
		BuyerUserID:    buyerID,
		CredentialID:   credential.ID,
		PurchasedQuota: purchasedQuota,
	})
	require.NoError(t, err)

	return marketplaceFixedOrderRelayFixture{
		DB:           db,
		BuyerUserID:  buyerID,
		SellerUserID: sellerID,
		Credential:   credential,
		Order:        order,
	}
}

func TestPrepareMarketplaceFixedOrderRelayReservesOrderQuotaOnly(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-reserve",
	})
	require.NoError(t, err)
	assert.Equal(t, fixture.Order.ID, preparation.Order.ID)
	assert.Equal(t, fixture.Credential.ID, preparation.Credential.ID)
	assert.Equal(t, "marketplace-service-key", preparation.APIKey)
	assert.Equal(t, 1000, preparation.Session.GetPreConsumedQuota())
	assert.True(t, preparation.Session.NeedsRefund())

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(1500), order.RemainingQuota)
	assert.Equal(t, int64(1000), order.SpentQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusActive, order.Status)

	var buyer model.User
	require.NoError(t, fixture.DB.First(&buyer, fixture.BuyerUserID).Error)
	assert.Equal(t, 7500, buyer.Quota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 1, stats.CurrentConcurrency)
}

func TestMarketplaceFixedOrderRelaySettleWritesFillStatsAndRealtimeSellerSettlement(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-settle",
	})
	require.NoError(t, err)

	require.NoError(t, preparation.Session.Settle(800))
	assert.False(t, preparation.Session.NeedsRefund())

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(1700), order.RemainingQuota)
	assert.Equal(t, int64(800), order.SpentQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusActive, order.Status)

	var fill model.MarketplaceFixedOrderFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "req-settle").Error)
	assert.Equal(t, fixture.Order.ID, fill.FixedOrderID)
	assert.Equal(t, fixture.BuyerUserID, fill.BuyerUserID)
	assert.Equal(t, fixture.SellerUserID, fill.SellerUserID)
	assert.Equal(t, fixture.Credential.ID, fill.CredentialID)
	assert.Equal(t, "gpt-4o-mini", fill.Model)
	assert.Equal(t, int64(640), fill.OfficialCost)
	assert.Equal(t, int64(800), fill.BuyerCharge)
	assert.Equal(t, 1.25, fill.MultiplierSnapshot)
	assert.Equal(t, model.MarketplaceFillStatusSucceeded, fill.Status)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-settle").Error)
	assert.Equal(t, marketplaceSettlementSourceFixedOrderFill, settlement.SourceType)
	assert.Equal(t, fmt.Sprintf("%d", fill.ID), settlement.SourceID)
	assert.Equal(t, int64(800), settlement.BuyerCharge)
	assert.Equal(t, int64(0), settlement.PlatformFee)
	assert.Equal(t, int64(800), settlement.SellerIncome)
	assert.Equal(t, int64(640), settlement.OfficialCost)
	assert.Equal(t, model.MarketplaceSettlementStatusAvailable, settlement.Status)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, int64(1), stats.FixedOrderRequestCount)
	assert.Equal(t, int64(1), stats.TotalRequestCount)
	assert.Equal(t, int64(640), stats.TotalOfficialCost)
	assert.Equal(t, int64(640), stats.QuotaUsed)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, 0, stats.CurrentConcurrency)
	assert.Greater(t, stats.LastSuccessAt, int64(0))

	var buyer model.User
	require.NoError(t, fixture.DB.First(&buyer, fixture.BuyerUserID).Error)
	assert.Equal(t, 7500, buyer.Quota)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 800, seller.Quota)

	var sellerLog model.Log
	require.NoError(t, fixture.DB.First(&sellerLog, "user_id = ? AND type = ?", fixture.SellerUserID, model.LogTypeSystem).Error)
	assert.Contains(t, sellerLog.Content, "市场买断收益实时结算")
	assert.Contains(t, sellerLog.Content, "＄0.001600")
}

func TestMarketplaceFixedOrderRelaySettleDeductsTransactionFeeFromBuyoutQuota(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixtureWithFee(t, 2500, 0.05)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-settle-fee",
	})
	require.NoError(t, err)
	assert.Equal(t, 1050, preparation.Session.GetPreConsumedQuota())

	var reservedOrder model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&reservedOrder, fixture.Order.ID).Error)
	assert.Equal(t, int64(1575), reservedOrder.RemainingQuota)
	assert.Equal(t, int64(1050), reservedOrder.SpentQuota)

	require.NoError(t, preparation.Session.Settle(800))

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(1785), order.RemainingQuota)
	assert.Equal(t, int64(840), order.SpentQuota)

	var fill model.MarketplaceFixedOrderFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "req-settle-fee").Error)
	assert.Equal(t, int64(840), fill.BuyerCharge)
	assert.Equal(t, int64(640), fill.OfficialCost)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-settle-fee").Error)
	assert.Equal(t, int64(840), settlement.BuyerCharge)
	assert.Equal(t, int64(40), settlement.PlatformFee)
	assert.Equal(t, 0.05, settlement.PlatformFeeRateSnapshot)
	assert.Equal(t, int64(800), settlement.SellerIncome)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 800, seller.Quota)
}

func TestMarketplaceFixedOrderRelayUsesBuyoutQuotaThatIncludesCreationFee(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixtureWithFee(t, 30*int64(common.QuotaPerUnit), 0.05)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 30 * int(common.QuotaPerUnit),
		RequestID:      "req-buyout-30-fee",
	})
	require.NoError(t, err)
	assert.Equal(t, 15750000, preparation.Session.GetPreConsumedQuota())

	require.NoError(t, preparation.Session.Settle(30*int(common.QuotaPerUnit)))

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(15750000), order.PurchasedQuota)
	assert.Equal(t, int64(0), order.RemainingQuota)
	assert.Equal(t, int64(15750000), order.SpentQuota)
	assert.Equal(t, 0.05, order.PlatformFeeRateSnapshot)
	assert.Equal(t, model.MarketplaceFixedOrderStatusExhausted, order.Status)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-buyout-30-fee").Error)
	assert.Equal(t, int64(15750000), settlement.BuyerCharge)
	assert.Equal(t, int64(750000), settlement.PlatformFee)
	assert.Equal(t, int64(15000000), settlement.SellerIncome)
	assert.Equal(t, 0.05, settlement.PlatformFeeRateSnapshot)
}

func TestMarketplaceFixedOrderRelayUsesOrderFeeSnapshotAfterGlobalRateChanges(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixtureWithFee(t, 30*int64(common.QuotaPerUnit), 0.05)
	setting.MarketplaceFeeRate = 0.10

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 30 * int(common.QuotaPerUnit),
		RequestID:      "req-buyout-rate-changed",
	})
	require.NoError(t, err)
	assert.Equal(t, 15750000, preparation.Session.GetPreConsumedQuota())

	require.NoError(t, preparation.Session.Settle(30*int(common.QuotaPerUnit)))

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(0), order.RemainingQuota)
	assert.Equal(t, int64(15750000), order.SpentQuota)
	assert.Equal(t, 0.05, order.PlatformFeeRateSnapshot)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-buyout-rate-changed").Error)
	assert.Equal(t, int64(15750000), settlement.BuyerCharge)
	assert.Equal(t, int64(750000), settlement.PlatformFee)
	assert.Equal(t, int64(15000000), settlement.SellerIncome)
	assert.Equal(t, 0.05, settlement.PlatformFeeRateSnapshot)
}

func TestMarketplaceFixedOrderRelayReserveUsesTransactionFeeForTargetComparison(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixtureWithFee(t, 2500, 0.05)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-reserve-fee-target",
	})
	require.NoError(t, err)
	require.NoError(t, preparation.Session.Reserve(1020))
	assert.Equal(t, 1071, preparation.Session.GetPreConsumedQuota())

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(1554), order.RemainingQuota)
	assert.Equal(t, int64(1071), order.SpentQuota)
}

func TestMarketplaceFixedOrderRelaySettleCapsBaseChargeWhenFeeConsumesBuyoutQuota(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixtureWithFee(t, 1000, 0.05)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 700,
		RequestID:      "req-exhaust-fee",
	})
	require.NoError(t, err)
	require.NoError(t, preparation.Session.Settle(1500))

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(0), order.RemainingQuota)
	assert.Equal(t, int64(1050), order.SpentQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusExhausted, order.Status)

	var fill model.MarketplaceFixedOrderFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "req-exhaust-fee").Error)
	assert.Equal(t, int64(1050), fill.BuyerCharge)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-exhaust-fee").Error)
	assert.Equal(t, int64(1050), settlement.BuyerCharge)
	assert.Equal(t, int64(50), settlement.PlatformFee)
	assert.Equal(t, int64(1000), settlement.SellerIncome)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 1000, seller.Quota)
}

func TestMarketplaceFixedOrderPurchaseRejectsOwnCredential(t *testing.T) {
	db := setupMarketplaceFixedOrderRelayTestDB(t)
	userID := 20
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "marketplace_self_buyer",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000,
		AffCode:  "self_buyer",
	}).Error)

	credential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     userID,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-self-service-key",
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

	order, err := CreateMarketplaceFixedOrder(MarketplaceFixedOrderCreateInput{
		BuyerUserID:    userID,
		CredentialID:   credential.ID,
		PurchasedQuota: 2500,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot buy own")
	require.Nil(t, order)

	var orderCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)

	var settlementCount int64
	require.NoError(t, db.Model(&model.MarketplaceSettlement{}).Count(&settlementCount).Error)
	assert.Equal(t, int64(0), settlementCount)

	var fillCount int64
	require.NoError(t, db.Model(&model.MarketplaceFixedOrderFill{}).Count(&fillCount).Error)
	assert.Equal(t, int64(0), fillCount)

	var buyer model.User
	require.NoError(t, db.First(&buyer, userID).Error)
	assert.Equal(t, 10000, buyer.Quota)
}

func TestMarketplaceFixedOrderRelayRefundRestoresReservedQuota(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-refund",
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	preparation.Session.Refund(ctx)

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(2500), order.RemainingQuota)
	assert.Equal(t, int64(0), order.SpentQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusActive, order.Status)
	assert.False(t, preparation.Session.NeedsRefund())

	var fillCount int64
	require.NoError(t, fixture.DB.Model(&model.MarketplaceFixedOrderFill{}).Count(&fillCount).Error)
	assert.Equal(t, int64(0), fillCount)

	var settlementCount int64
	require.NoError(t, fixture.DB.Model(&model.MarketplaceSettlement{}).Count(&settlementCount).Error)
	assert.Equal(t, int64(0), settlementCount)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 0, stats.CurrentConcurrency)
}

func TestMarketplaceFixedOrderRelayRefundReleasesCapacityWithoutReservedQuota(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 0,
		RequestID:      "req-refund-capacity",
	})
	require.NoError(t, err)
	require.True(t, preparation.Session.NeedsRefund())

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	preparation.Session.Refund(ctx)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, 0, stats.CurrentConcurrency)
	assert.False(t, preparation.Session.NeedsRefund())
}

func TestMarketplaceFixedOrderRelaySettleExhaustsOrder(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 1000)
	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 700,
		RequestID:      "req-exhaust",
	})
	require.NoError(t, err)
	require.NoError(t, preparation.Session.Settle(1500))

	var order model.MarketplaceFixedOrder
	require.NoError(t, fixture.DB.First(&order, fixture.Order.ID).Error)
	assert.Equal(t, int64(0), order.RemainingQuota)
	assert.Equal(t, int64(1000), order.SpentQuota)
	assert.Equal(t, model.MarketplaceFixedOrderStatusExhausted, order.Status)

	var fill model.MarketplaceFixedOrderFill
	require.NoError(t, fixture.DB.First(&fill, "request_id = ?", "req-exhaust").Error)
	assert.Equal(t, int64(1000), fill.BuyerCharge)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", "req-exhaust").Error)
	assert.Equal(t, marketplaceSettlementSourceFixedOrderFill, settlement.SourceType)
	assert.Equal(t, int64(1000), settlement.BuyerCharge)
	assert.Equal(t, int64(0), settlement.PlatformFee)
	assert.Equal(t, int64(1000), settlement.SellerIncome)

	var finalSettlementCount int64
	require.NoError(t, fixture.DB.Model(&model.MarketplaceSettlement{}).
		Where("source_type = ?", marketplaceSettlementSourceFixedOrderFinal).
		Count(&finalSettlementCount).Error)
	assert.Equal(t, int64(0), finalSettlementCount)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 1000, seller.Quota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, int64(0), stats.ActiveFixedOrderCount)
}

func TestMarketplaceFixedOrderRelaySettleExhaustsLimitedCredential(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 1000)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", fixture.Credential.ID).
		Updates(map[string]any{
			"quota_mode":  model.MarketplaceQuotaModeLimited,
			"quota_limit": int64(700),
		}).Error)

	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 700,
		RequestID:      "req-credential-exhaust",
	})
	require.NoError(t, err)
	require.NoError(t, preparation.Session.Settle(1000))

	var credential model.MarketplaceCredential
	require.NoError(t, fixture.DB.First(&credential, fixture.Credential.ID).Error)
	assert.Equal(t, model.MarketplaceCapacityStatusExhausted, credential.CapacityStatus)
}

func TestPrepareMarketplaceFixedOrderRelayRejectsBusyCredential(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", fixture.Credential.ID).
		Update("concurrency_limit", 1).Error)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", fixture.Credential.ID).
		Update("current_concurrency", 1).Error)

	_, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 100,
		RequestID:      "req-busy",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "busy")
}

func TestPrepareMarketplaceFixedOrderRelayRejectsStatsExhaustedCredential(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", fixture.Credential.ID).
		Updates(map[string]any{
			"quota_mode":      model.MarketplaceQuotaModeLimited,
			"quota_limit":     int64(1000),
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
		}).Error)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredentialStats{}).
		Where("credential_id = ?", fixture.Credential.ID).
		Update("quota_used", 1000).Error)

	_, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 100,
		RequestID:      "req-stats-exhausted",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestListBuyerMarketplaceFixedOrdersExpiresDueOrders(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	preparation, err := PrepareMarketplaceFixedOrderRelay(MarketplaceFixedOrderRelayInput{
		BuyerUserID:    fixture.BuyerUserID,
		FixedOrderID:   fixture.Order.ID,
		Model:          "gpt-4o-mini",
		EstimatedQuota: 1000,
		RequestID:      "req-final-before-expire",
	})
	require.NoError(t, err)
	require.NoError(t, preparation.Session.Settle(800))

	require.NoError(t, fixture.DB.Model(&model.MarketplaceFixedOrder{}).
		Where("id = ?", fixture.Order.ID).
		Update("expires_at", common.GetTimestamp()-1).Error)

	orders, total, err := ListBuyerMarketplaceFixedOrders(fixture.BuyerUserID, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, orders, 1)
	assert.Equal(t, model.MarketplaceFixedOrderStatusExpired, orders[0].Status)
	assert.Equal(t, int64(0), orders[0].RemainingQuota)

	var stats model.MarketplaceCredentialStats
	require.NoError(t, fixture.DB.First(&stats, "credential_id = ?", fixture.Credential.ID).Error)
	assert.Equal(t, int64(0), stats.ActiveFixedOrderCount)

	var settlement model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&settlement, "request_id = ?", marketplaceFixedOrderFinalSettlementRequestID(fixture.Order.ID)).Error)
	assert.Equal(t, marketplaceSettlementSourceFixedOrderFinal, settlement.SourceType)
	assert.Equal(t, fmt.Sprintf("%d", fixture.Order.ID), settlement.SourceID)
	assert.Equal(t, int64(1700), settlement.BuyerCharge)
	assert.Equal(t, int64(0), settlement.PlatformFee)
	assert.Equal(t, int64(1700), settlement.SellerIncome)
	assert.Equal(t, int64(0), settlement.OfficialCost)
	assert.Equal(t, model.MarketplaceSettlementStatusAvailable, settlement.Status)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 2500, seller.Quota)

	var finalLog model.Log
	require.NoError(t, fixture.DB.
		Where("user_id = ? AND type = ? AND content LIKE ?", fixture.SellerUserID, model.LogTypeSystem, "%市场买断剩余额度结算%").
		First(&finalLog).Error)
	assert.Contains(t, finalLog.Content, "＄0.003400")
	assert.Contains(t, finalLog.Content, "托管Key")

	orders, total, err = ListBuyerMarketplaceFixedOrders(fixture.BuyerUserID, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, orders, 1)

	var settlementCount int64
	require.NoError(t, fixture.DB.Model(&model.MarketplaceSettlement{}).Count(&settlementCount).Error)
	assert.Equal(t, int64(2), settlementCount)
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 2500, seller.Quota)
}

func TestSelectBuyerMarketplaceFixedOrderForTokenBindingsChoosesSupportedModel(t *testing.T) {
	fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
	secondCredential, err := CreateSellerMarketplaceCredential(MarketplaceCredentialCreateInput{
		SellerUserID:     fixture.SellerUserID,
		VendorType:       constant.ChannelTypeOpenAI,
		APIKey:           "marketplace-service-key-two",
		Models:           []string{"gpt-4o"},
		QuotaMode:        model.MarketplaceQuotaModeUnlimited,
		Multiplier:       1.5,
		ConcurrencyLimit: 2,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", secondCredential.ID).
		Updates(map[string]any{
			"health_status":   model.MarketplaceHealthStatusHealthy,
			"capacity_status": model.MarketplaceCapacityStatusAvailable,
			"risk_status":     model.MarketplaceRiskStatusNormal,
		}).Error)
	secondOrder, err := CreateMarketplaceFixedOrder(MarketplaceFixedOrderCreateInput{
		BuyerUserID:    fixture.BuyerUserID,
		CredentialID:   secondCredential.ID,
		PurchasedQuota: 2500,
	})
	require.NoError(t, err)

	selected, err := SelectBuyerMarketplaceFixedOrderForTokenBindings(MarketplaceFixedOrderBindingSelectInput{
		BuyerUserID:   fixture.BuyerUserID,
		FixedOrderIDs: []int{fixture.Order.ID, secondOrder.ID},
		Model:         "gpt-4o",
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, secondOrder.ID, selected.ID)
}

func TestPrepareMarketplaceFixedOrderRelayRejectsInvalidOrders(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, fixture marketplaceFixedOrderRelayFixture)
		input   func(fixture marketplaceFixedOrderRelayFixture) MarketplaceFixedOrderRelayInput
		wantErr string
	}{
		{
			name: "non owner",
			input: func(fixture marketplaceFixedOrderRelayFixture) MarketplaceFixedOrderRelayInput {
				return MarketplaceFixedOrderRelayInput{
					BuyerUserID:    21,
					FixedOrderID:   fixture.Order.ID,
					Model:          "gpt-4o-mini",
					EstimatedQuota: 100,
					RequestID:      "req-non-owner",
				}
			},
			wantErr: "not found",
		},
		{
			name: "expired",
			mutate: func(t *testing.T, fixture marketplaceFixedOrderRelayFixture) {
				t.Helper()
				require.NoError(t, fixture.DB.Model(&model.MarketplaceFixedOrder{}).
					Where("id = ?", fixture.Order.ID).
					Update("expires_at", common.GetTimestamp()-1).Error)
			},
			input: func(fixture marketplaceFixedOrderRelayFixture) MarketplaceFixedOrderRelayInput {
				return MarketplaceFixedOrderRelayInput{
					BuyerUserID:    fixture.BuyerUserID,
					FixedOrderID:   fixture.Order.ID,
					Model:          "gpt-4o-mini",
					EstimatedQuota: 100,
					RequestID:      "req-expired",
				}
			},
			wantErr: "expired",
		},
		{
			name: "disabled credential",
			mutate: func(t *testing.T, fixture marketplaceFixedOrderRelayFixture) {
				t.Helper()
				require.NoError(t, fixture.DB.Model(&model.MarketplaceCredential{}).
					Where("id = ?", fixture.Credential.ID).
					Update("service_status", model.MarketplaceServiceStatusDisabled).Error)
			},
			input: func(fixture marketplaceFixedOrderRelayFixture) MarketplaceFixedOrderRelayInput {
				return MarketplaceFixedOrderRelayInput{
					BuyerUserID:    fixture.BuyerUserID,
					FixedOrderID:   fixture.Order.ID,
					Model:          "gpt-4o-mini",
					EstimatedQuota: 100,
					RequestID:      "req-disabled",
				}
			},
			wantErr: "not available",
		},
		{
			name: "unsupported model",
			input: func(fixture marketplaceFixedOrderRelayFixture) MarketplaceFixedOrderRelayInput {
				return MarketplaceFixedOrderRelayInput{
					BuyerUserID:    fixture.BuyerUserID,
					FixedOrderID:   fixture.Order.ID,
					Model:          "gpt-4o",
					EstimatedQuota: 100,
					RequestID:      "req-model",
				}
			},
			wantErr: "model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newMarketplaceFixedOrderRelayFixture(t, 2500)
			if tt.mutate != nil {
				tt.mutate(t, fixture)
			}
			_, err := PrepareMarketplaceFixedOrderRelay(tt.input(fixture))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
