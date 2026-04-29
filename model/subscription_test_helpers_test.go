package model

import (
	"fmt"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
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
		&ReferralTemplate{},
		&ReferralTemplateBinding{},
		&ReferralInviteeShareOverride{},
		&ReferralSettlementBatch{},
		&ReferralSettlementRecord{},
		&TopUp{},
		&CryptoPaymentOrder{},
		&CryptoPaymentTransaction{},
		&CryptoScannerState{},
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

func seedPendingReferralOrder(t *testing.T, db *gorm.DB, userID int, planID int, tradeNo string, amount float64) *SubscriptionOrder {
	t.Helper()

	order := &SubscriptionOrder{
		UserId:        userID,
		PlanId:        planID,
		Money:         amount,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create pending order: %v", err)
	}
	return order
}
