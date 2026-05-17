package model

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupWithdrawalModelDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&Option{}, &User{}, &UserWithdrawal{}, &Log{}, &Redemption{}); err != nil {
		t.Fatalf("failed to migrate withdrawal tables: %v", err)
	}
	InitOptionMap()

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
		common.OptionMap = originalOptionMap
	})

	return db
}

func seedWithdrawalUser(t *testing.T, db *gorm.DB, username string, quota int) *User {
	t.Helper()

	user := &User{
		Username: username,
		Password: "password",
		AffCode:  username + "_code",
		Group:    "default",
		Quota:    quota,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create withdrawal user: %v", err)
	}
	return user
}

func seedRedeemedQuota(t *testing.T, db *gorm.DB, userID int, quota int) *Redemption {
	t.Helper()

	redemption := &Redemption{
		UserId:       userID,
		Key:          fmt.Sprintf("redeemed_%d_%d", userID, quota),
		Status:       common.RedemptionCodeStatusUsed,
		Name:         "redeemed quota",
		Quota:        quota,
		CreatedTime:  common.GetTimestamp(),
		RedeemedTime: common.GetTimestamp(),
		UsedUserId:   userID,
	}
	if err := db.Create(redemption).Error; err != nil {
		t.Fatalf("failed to create redeemed quota: %v", err)
	}
	return redemption
}

func seedPendingWithdrawal(t *testing.T, db *gorm.DB, userID int, amount float64) *UserWithdrawal {
	t.Helper()

	applyQuota := int(decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart())
	feeQuota := int(decimal.NewFromFloat(2).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart())
	netQuota := int(decimal.NewFromFloat(amount - 2).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart())

	withdrawal := &UserWithdrawal{
		UserId:                 userID,
		TradeNo:                "WDR-PENDING-" + strconv.Itoa(userID),
		Channel:                WithdrawalChannelAlipay,
		Currency:               "CNY",
		ExchangeRateSnapshot:   1,
		AvailableQuotaSnapshot: 100000,
		FrozenQuotaSnapshot:    applyQuota,
		ApplyAmount:            amount,
		FeeAmount:              2,
		NetAmount:              amount - 2,
		ApplyQuota:             applyQuota,
		FeeQuota:               feeQuota,
		NetQuota:               netQuota,
		AlipayAccount:          "alice@example.com",
		AlipayRealName:         "Alice",
		Status:                 UserWithdrawalStatusPending,
	}
	if err := db.Create(withdrawal).Error; err != nil {
		t.Fatalf("failed to create pending withdrawal: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"quota":                 gorm.Expr("quota - ?", applyQuota),
		"withdraw_frozen_quota": gorm.Expr("withdraw_frozen_quota + ?", applyQuota),
	}).Error; err != nil {
		t.Fatalf("failed to freeze quota for pending withdrawal: %v", err)
	}
	return withdrawal
}

func seedApprovedWithdrawal(t *testing.T, db *gorm.DB, userID int, amount float64) *UserWithdrawal {
	t.Helper()

	withdrawal := seedPendingWithdrawal(t, db, userID, amount)
	withdrawal.Status = UserWithdrawalStatusApproved
	if err := db.Save(withdrawal).Error; err != nil {
		t.Fatalf("failed to mark withdrawal approved: %v", err)
	}
	return withdrawal
}
