package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSubscriptionReferralSettlementDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	originalCommonGroupCol := commonGroupCol

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	commonGroupCol = "`group`"

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(
		&User{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionReferralOverride{},
		&SubscriptionReferralRecord{},
		&TopUp{},
		&Log{},
	); err != nil {
		t.Fatalf("failed to migrate settlement tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
		commonGroupCol = originalCommonGroupCol
	})

	return db
}

func seedReferralUser(t *testing.T, db *gorm.DB, username string, inviterID int, setting dto.UserSetting) *User {
	t.Helper()

	user := &User{
		Username:  username,
		Password:  "password",
		AffCode:   username + "_code",
		Group:     "default",
		InviterId: inviterID,
	}
	user.SetSetting(setting)
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func seedReferralPlan(t *testing.T, db *gorm.DB, price float64) *SubscriptionPlan {
	t.Helper()

	plan := &SubscriptionPlan{
		Title:         "Referral Plan",
		PriceAmount:   price,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}
	InvalidateSubscriptionPlanCache(plan.Id)
	return plan
}

func setReferralPlanUpgradeGroup(t *testing.T, db *gorm.DB, plan *SubscriptionPlan, group string) {
	t.Helper()

	plan.UpgradeGroup = group
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update plan upgrade group: %v", err)
	}
	InvalidateSubscriptionPlanCache(plan.Id)
}

func seedPendingReferralOrder(t *testing.T, db *gorm.DB, userID, planID int, tradeNo string, money float64) *SubscriptionOrder {
	t.Helper()

	order := &SubscriptionOrder{
		UserId:        userID,
		PlanId:        planID,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	return order
}

func TestCompleteSubscriptionOrderWithoutInviterSkipsReferralRecords(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	user := seedReferralUser(t, db, "standalone", 0, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	order := seedPendingReferralOrder(t, db, user.Id, plan.Id, "trade-ref-no-inviter", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	after, _ := GetUserById(user.Id, true)
	if after.AffQuota != 0 || after.AffHistoryQuota != 0 {
		t.Fatalf("expected no referral quota for standalone user, got aff_quota=%d aff_history=%d", after.AffQuota, after.AffHistoryQuota)
	}

	var count int64
	if err := db.Model(&SubscriptionReferralRecord{}).Where("order_trade_no = ?", order.TradeNo).Count(&count).Error; err != nil {
		t.Fatalf("failed to count referral records: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 referral records, got %d", count)
	}
}

func TestCompleteSubscriptionOrderWithoutUpgradeGroupSkipsReferralRecords(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	inviter := seedReferralUser(t, db, "inviter-no-upgrade-group", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	invitee := seedReferralUser(t, db, "invitee-no-upgrade-group", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-no-upgrade-group", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	inviterAfter, _ := GetUserById(inviter.Id, true)
	inviteeAfter, _ := GetUserById(invitee.Id, true)
	if inviterAfter.AffQuota != 0 || inviteeAfter.AffQuota != 0 {
		t.Fatalf("expected no referral quota when plan upgrade group is empty, got inviter=%d invitee=%d", inviterAfter.AffQuota, inviteeAfter.AffQuota)
	}

	var count int64
	if err := db.Model(&SubscriptionReferralRecord{}).Where("order_trade_no = ?", order.TradeNo).Count(&count).Error; err != nil {
		t.Fatalf("failed to count referral records: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 referral records, got %d", count)
	}
}

func TestCompleteSubscriptionOrderCreditsInviterAndInviteeReferral(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	inviter := seedReferralUser(t, db, "inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	invitee := seedReferralUser(t, db, "invitee", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "default")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-1", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	inviterAfter, _ := GetUserById(inviter.Id, true)
	inviteeAfter, _ := GetUserById(invitee.Id, true)
	if inviterAfter.AffQuota != 150 || inviteeAfter.AffQuota != 50 {
		t.Fatalf("unexpected quotas inviter=%d invitee=%d", inviterAfter.AffQuota, inviteeAfter.AffQuota)
	}

	var count int64
	if err := db.Model(&SubscriptionReferralRecord{}).Where("order_trade_no = ?", order.TradeNo).Count(&count).Error; err != nil {
		t.Fatalf("failed to count referral records: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 referral records, got %d", count)
	}
}

func TestCompleteSubscriptionOrderCreditsConfiguredGroupReferral(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":4500}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	inviter := seedReferralUser(t, db, "group-config-inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 500},
	})
	invitee := seedReferralUser(t, db, "group-config-invitee", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "vip")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-group-configured", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	inviterAfter, _ := GetUserById(inviter.Id, true)
	inviteeAfter, _ := GetUserById(invitee.Id, true)
	if inviterAfter.AffQuota != 400 || inviteeAfter.AffQuota != 50 {
		t.Fatalf("unexpected quotas inviter=%d invitee=%d", inviterAfter.AffQuota, inviteeAfter.AffQuota)
	}

	var records []SubscriptionReferralRecord
	if err := db.Where("order_trade_no = ?", order.TradeNo).Order("beneficiary_role asc").Find(&records).Error; err != nil {
		t.Fatalf("failed to load referral records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 referral records, got %d", len(records))
	}
	for _, record := range records {
		if record.ReferralGroup != "vip" {
			t.Fatalf("ReferralGroup = %q, want vip", record.ReferralGroup)
		}
		if record.TotalRateBpsSnapshot != 4500 {
			t.Fatalf("TotalRateBpsSnapshot = %d, want 4500", record.TotalRateBpsSnapshot)
		}
		if record.InviteeRateBpsSnapshot != 500 {
			t.Fatalf("InviteeRateBpsSnapshot = %d, want 500", record.InviteeRateBpsSnapshot)
		}
	}
}

func TestCompleteSubscriptionOrderUsesGroupedOverrideRate(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":4500}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	inviter := seedReferralUser(t, db, "group-override-inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 500},
	})
	if _, err := UpsertSubscriptionReferralOverride(inviter.Id, "vip", 2000, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}

	invitee := seedReferralUser(t, db, "group-override-invitee", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "vip")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-group-override", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	inviterAfter, _ := GetUserById(inviter.Id, true)
	inviteeAfter, _ := GetUserById(invitee.Id, true)
	if inviterAfter.AffQuota != 150 || inviteeAfter.AffQuota != 50 {
		t.Fatalf("unexpected quotas inviter=%d invitee=%d", inviterAfter.AffQuota, inviteeAfter.AffQuota)
	}

	var inviterRecord SubscriptionReferralRecord
	if err := db.Where("order_trade_no = ? AND beneficiary_role = ?", order.TradeNo, SubscriptionReferralBeneficiaryRoleInviter).First(&inviterRecord).Error; err != nil {
		t.Fatalf("failed to load inviter referral record: %v", err)
	}
	if inviterRecord.ReferralGroup != "vip" {
		t.Fatalf("ReferralGroup = %q, want vip", inviterRecord.ReferralGroup)
	}
	if inviterRecord.TotalRateBpsSnapshot != 2000 {
		t.Fatalf("TotalRateBpsSnapshot = %d, want 2000", inviterRecord.TotalRateBpsSnapshot)
	}
	if inviterRecord.InviteeRateBpsSnapshot != 500 {
		t.Fatalf("InviteeRateBpsSnapshot = %d, want 500", inviterRecord.InviteeRateBpsSnapshot)
	}
}

func TestCompleteSubscriptionOrderIgnoresInviteeCurrentGroupForReferralRates(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":3000}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	inviter := seedReferralUser(t, db, "group-agnostic-inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	if _, err := UpsertSubscriptionReferralOverride(inviter.Id, "default", 2500, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}

	invitee := seedReferralUser(t, db, "group-agnostic-invitee", inviter.Id, dto.UserSetting{})
	if err := db.Model(&User{}).Where("id = ?", invitee.Id).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update invitee group: %v", err)
	}
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "default")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-ignore-current-group", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}

	var inviterRecord SubscriptionReferralRecord
	if err := db.Where("order_trade_no = ? AND beneficiary_role = ?", order.TradeNo, SubscriptionReferralBeneficiaryRoleInviter).First(&inviterRecord).Error; err != nil {
		t.Fatalf("failed to load inviter referral record: %v", err)
	}
	if inviterRecord.TotalRateBpsSnapshot != 2500 {
		t.Fatalf("TotalRateBpsSnapshot = %d, want 2500", inviterRecord.TotalRateBpsSnapshot)
	}
	if inviterRecord.AppliedRateBps != 2000 {
		t.Fatalf("AppliedRateBps = %d, want 2000", inviterRecord.AppliedRateBps)
	}
	if inviterRecord.RewardQuota != 200 {
		t.Fatalf("RewardQuota = %d, want 200", inviterRecord.RewardQuota)
	}
}

func TestCompleteSubscriptionOrderIsIdempotentForReferralRecords(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	inviter := seedReferralUser(t, db, "inviter-idempotent", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	invitee := seedReferralUser(t, db, "invitee-idempotent", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "default")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-2", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("first CompleteSubscriptionOrder returned error: %v", err)
	}
	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("second CompleteSubscriptionOrder returned error: %v", err)
	}

	var count int64
	if err := db.Model(&SubscriptionReferralRecord{}).Where("order_trade_no = ?", order.TradeNo).Count(&count).Error; err != nil {
		t.Fatalf("failed to count referral rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected exactly two referral rows, got %d", count)
	}

	inviterAfter, _ := GetUserById(inviter.Id, true)
	inviteeAfter, _ := GetUserById(invitee.Id, true)
	if inviterAfter.AffQuota != 150 || inviteeAfter.AffQuota != 50 {
		t.Fatalf("unexpected quotas inviter=%d invitee=%d", inviterAfter.AffQuota, inviteeAfter.AffQuota)
	}
}

func TestReverseSubscriptionReferralByTradeNoCreatesDebtWhenAffQuotaIsInsufficient(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalEnabled := common.SubscriptionReferralEnabled
	originalRate := common.SubscriptionReferralGlobalRateBps
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalRate
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	inviter := seedReferralUser(t, db, "inviter-reverse", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	invitee := seedReferralUser(t, db, "invitee-reverse", inviter.Id, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 10)
	setReferralPlanUpgradeGroup(t, db, plan, "default")
	order := seedPendingReferralOrder(t, db, invitee.Id, plan.Id, "trade-ref-3", 10)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder returned error: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", invitee.Id).Update("aff_quota", 10).Error; err != nil {
		t.Fatalf("failed to reduce invitee aff_quota: %v", err)
	}

	if err := ReverseSubscriptionReferralByTradeNo(order.TradeNo, 1); err != nil {
		t.Fatalf("ReverseSubscriptionReferralByTradeNo returned error: %v", err)
	}

	var inviteeRecord SubscriptionReferralRecord
	if err := db.Where("order_trade_no = ? AND beneficiary_role = ?", order.TradeNo, SubscriptionReferralBeneficiaryRoleInvitee).First(&inviteeRecord).Error; err != nil {
		t.Fatalf("failed to load invitee referral record: %v", err)
	}
	if inviteeRecord.ReversedQuota != 10 || inviteeRecord.DebtQuota != 40 {
		t.Fatalf("unexpected reversal state: %+v", inviteeRecord)
	}
}

func TestReverseSubscriptionReferralByTradeNoRejectsUnknownTradeNo(t *testing.T) {
	setupSubscriptionReferralSettlementDB(t)

	if err := ReverseSubscriptionReferralByTradeNo("missing-trade-no", 1); err == nil {
		t.Fatal("expected missing trade_no to return an error")
	}
}
