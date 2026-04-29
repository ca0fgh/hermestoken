package service

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSellerMarketplaceIncomeSummaryAndRelease(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	now := common.GetTimestamp()

	due := seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "due-1", 760, model.MarketplaceSettlementStatusPending, now-1)
	_ = seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "due-2", 300, model.MarketplaceSettlementStatusPending, now)
	future := seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "future", 500, model.MarketplaceSettlementStatusPending, now+3600)
	blocked := seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "blocked", 900, model.MarketplaceSettlementStatusBlocked, now-1)
	otherSeller := seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, 99, fixture.Credential.ID, "other", 700, model.MarketplaceSettlementStatusPending, now-1)
	require.NoError(t, fixture.DB.Create(&model.User{
		Id:       99,
		Username: "marketplace_other_seller",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "marketplace_other_seller",
	}).Error)

	summary, err := GetSellerMarketplaceIncomeSummary(fixture.SellerUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(1560), summary.PendingIncome)
	assert.Equal(t, int64(0), summary.AvailableIncome)
	assert.Equal(t, int64(900), summary.BlockedIncome)
	assert.Equal(t, int64(2460), summary.TotalSellerIncome)
	assert.Equal(t, int64(4), summary.SettlementCount)

	result, err := ReleaseSellerAvailableMarketplaceSettlements(fixture.SellerUserID, now, 100)
	require.NoError(t, err)
	assert.Equal(t, 2, result.ReleasedCount)
	assert.Equal(t, int64(1060), result.ReleasedIncome)

	var seller model.User
	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 1060, seller.Quota)

	var refreshedDue model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&refreshedDue, due.ID).Error)
	assert.Equal(t, model.MarketplaceSettlementStatusAvailable, refreshedDue.Status)

	var refreshedFuture model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&refreshedFuture, future.ID).Error)
	assert.Equal(t, model.MarketplaceSettlementStatusPending, refreshedFuture.Status)

	var refreshedBlocked model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&refreshedBlocked, blocked.ID).Error)
	assert.Equal(t, model.MarketplaceSettlementStatusBlocked, refreshedBlocked.Status)

	var refreshedOther model.MarketplaceSettlement
	require.NoError(t, fixture.DB.First(&refreshedOther, otherSeller.ID).Error)
	assert.Equal(t, model.MarketplaceSettlementStatusPending, refreshedOther.Status)

	second, err := ReleaseSellerAvailableMarketplaceSettlements(fixture.SellerUserID, now, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, second.ReleasedCount)
	assert.Equal(t, int64(0), second.ReleasedIncome)

	require.NoError(t, fixture.DB.First(&seller, fixture.SellerUserID).Error)
	assert.Equal(t, 1060, seller.Quota)

	summary, err = GetSellerMarketplaceIncomeSummary(fixture.SellerUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), summary.PendingIncome)
	assert.Equal(t, int64(1060), summary.AvailableIncome)
	assert.Equal(t, int64(900), summary.BlockedIncome)
	assert.Equal(t, int64(2460), summary.TotalSellerIncome)
}

func TestListSellerMarketplaceSettlementsIsOwnerScopedAndFiltered(t *testing.T) {
	fixture := newMarketplacePoolRelayFixture(t)
	now := common.GetTimestamp()
	available := seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "available", 760, model.MarketplaceSettlementStatusAvailable, now-1)
	_ = seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, fixture.SellerUserID, fixture.Credential.ID, "pending", 300, model.MarketplaceSettlementStatusPending, now+3600)
	_ = seedMarketplaceSettlement(t, fixture.DB, fixture.BuyerUserID, 99, fixture.Credential.ID, "other", 700, model.MarketplaceSettlementStatusAvailable, now-1)

	items, total, err := ListSellerMarketplaceSettlements(MarketplaceSettlementListInput{
		SellerUserID: fixture.SellerUserID,
		Status:       model.MarketplaceSettlementStatusAvailable,
	}, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, available.ID, items[0].ID)
	assert.Equal(t, fixture.SellerUserID, items[0].SellerUserID)
	assert.Equal(t, model.MarketplaceSettlementStatusAvailable, items[0].Status)
}

func seedMarketplaceSettlement(t *testing.T, db *gorm.DB, buyerID int, sellerID int, credentialID int, requestID string, sellerIncome int64, status string, availableAt int64) model.MarketplaceSettlement {
	t.Helper()

	settlement := model.MarketplaceSettlement{
		RequestID:               requestID,
		BuyerUserID:             buyerID,
		SellerUserID:            sellerID,
		CredentialID:            credentialID,
		SourceType:              "test",
		SourceID:                requestID,
		BuyerCharge:             sellerIncome,
		PlatformFee:             0,
		PlatformFeeRateSnapshot: 0,
		SellerIncome:            sellerIncome,
		OfficialCost:            sellerIncome,
		MultiplierSnapshot:      1,
		Status:                  status,
		AvailableAt:             availableAt,
	}
	require.NoError(t, db.Create(&settlement).Error)
	return settlement
}
