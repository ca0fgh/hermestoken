package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var _ func(int, string, int, int) (*SubscriptionReferralOverride, error) = UpsertSubscriptionReferralOverride

type legacySubscriptionReferralOverrideSchemaRow struct {
	Id           int    `gorm:"primaryKey"`
	UserId       int    `gorm:"index:idx_subscription_referral_override_user_group"`
	Group        string `gorm:"type:varchar(64);not null;default:'';index:idx_subscription_referral_override_user_group"`
	TotalRateBps int    `gorm:"type:int;not null;default:0"`
	CreatedBy    int    `gorm:"type:int;not null;default:0"`
	UpdatedBy    int    `gorm:"type:int;not null;default:0"`
	CreatedAt    int64  `gorm:"bigint"`
	UpdatedAt    int64  `gorm:"bigint"`
}

func (legacySubscriptionReferralOverrideSchemaRow) TableName() string {
	return "subscription_referral_overrides"
}

type legacySubscriptionReferralRecordSchemaRow struct {
	Id                     int     `gorm:"primaryKey"`
	OrderId                int     `gorm:"index;uniqueIndex:idx_sub_referral_once"`
	OrderTradeNo           string  `gorm:"type:varchar(255);index"`
	PlanId                 int     `gorm:"index"`
	PayerUserId            int     `gorm:"index"`
	InviterUserId          int     `gorm:"index"`
	BeneficiaryUserId      int     `gorm:"index;uniqueIndex:idx_sub_referral_once"`
	BeneficiaryRole        string  `gorm:"type:varchar(16);uniqueIndex:idx_sub_referral_once"`
	OrderPaidAmount        float64 `gorm:"type:decimal(10,6);not null;default:0"`
	QuotaPerUnitSnapshot   float64 `gorm:"type:decimal(18,6);not null;default:0"`
	TotalRateBpsSnapshot   int     `gorm:"type:int;not null;default:0"`
	InviteeRateBpsSnapshot int     `gorm:"type:int;not null;default:0"`
	AppliedRateBps         int     `gorm:"type:int;not null;default:0"`
	RewardQuota            int64   `gorm:"type:bigint;not null;default:0"`
	ReversedQuota          int64   `gorm:"type:bigint;not null;default:0"`
	DebtQuota              int64   `gorm:"type:bigint;not null;default:0"`
	Status                 string  `gorm:"type:varchar(32);not null;default:'credited';index"`
	CreatedAt              int64   `gorm:"bigint"`
	UpdatedAt              int64   `gorm:"bigint;index"`
}

func (legacySubscriptionReferralRecordSchemaRow) TableName() string {
	return "subscription_referral_records"
}

func setupSubscriptionReferralOverrideSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&legacySubscriptionReferralOverrideSchemaRow{}); err != nil {
		t.Fatalf("failed to create legacy override table: %v", err)
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
	})

	return db
}

func setupSubscriptionReferralRecordSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&legacySubscriptionReferralRecordSchemaRow{}); err != nil {
		t.Fatalf("failed to create legacy referral record table: %v", err)
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
	})

	return db
}

func TestNormalizeSubscriptionReferralRateBps(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "negative clamps to zero", input: -1, want: 0},
		{name: "zero stays zero", input: 0, want: 0},
		{name: "middle stays same", input: 2500, want: 2500},
		{name: "max stays same", input: 10000, want: 10000},
		{name: "over max clamps", input: 12000, want: 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSubscriptionReferralRateBps(tt.input)
			if got != tt.want {
				t.Fatalf("NormalizeSubscriptionReferralRateBps(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveSubscriptionReferralConfigPrefersOverrideAndClampsInviteeRate(t *testing.T) {
	originalEnabled := common.SubscriptionReferralEnabled
	originalGlobal := common.SubscriptionReferralGlobalRateBps
	t.Cleanup(func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalGlobal
	})

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000

	setting := dto.UserSetting{
		SubscriptionReferralInviteeRateBps: 7000,
	}

	cfg := ResolveSubscriptionReferralConfig(3500, setting.SubscriptionReferralInviteeRateBps)
	if !cfg.Enabled {
		t.Fatal("expected referral config to stay enabled")
	}
	if cfg.TotalRateBps != 3500 {
		t.Fatalf("unexpected total rate: %d", cfg.TotalRateBps)
	}
	if cfg.InviteeRateBps != 3500 {
		t.Fatalf("unexpected invitee rate: %d", cfg.InviteeRateBps)
	}
	if cfg.InviterRateBps != 0 {
		t.Fatalf("unexpected inviter rate: %d", cfg.InviterRateBps)
	}

	cfg = ResolveSubscriptionReferralConfig(1800, setting.SubscriptionReferralInviteeRateBps)
	if cfg.InviteeRateBps != 1800 || cfg.InviterRateBps != 0 {
		t.Fatalf("expected clamped config, got %+v", cfg)
	}
}

func TestCalculateSubscriptionReferralQuotaUsesMoneyAndQuotaPerUnit(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.QuotaPerUnit = 500000

	got := CalculateSubscriptionReferralQuota(10, 2000)
	want := 1000000
	if got != want {
		t.Fatalf("CalculateSubscriptionReferralQuota() = %d, want %d", got, want)
	}
}

func TestSubscriptionReferralGroupRatesJSONRoundTrip(t *testing.T) {
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"default":4500,"vip":3000}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	if got := common.GetSubscriptionReferralGroupRate("default"); got != 4500 {
		t.Fatalf("GetSubscriptionReferralGroupRate(default) = %d, want 4500", got)
	}
	if got := common.GetSubscriptionReferralGroupRate("missing"); got != 0 {
		t.Fatalf("GetSubscriptionReferralGroupRate(missing) = %d, want 0", got)
	}
	if !common.HasSubscriptionReferralGroupRatesConfigured() {
		t.Fatal("expected group rates to be configured")
	}
}

func TestGetEffectiveSubscriptionReferralInviteeRateBpsByGroupFallsBackToLegacyValue(t *testing.T) {
	setting := dto.UserSetting{
		SubscriptionReferralInviteeRateBps:        700,
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 900},
	}

	if got := GetEffectiveSubscriptionReferralInviteeRateBps(setting, "vip", 1500); got != 900 {
		t.Fatalf("GetEffectiveSubscriptionReferralInviteeRateBps(vip, 1500) = %d, want 900", got)
	}
	if got := GetEffectiveSubscriptionReferralInviteeRateBps(setting, "default", 1500); got != 700 {
		t.Fatalf("GetEffectiveSubscriptionReferralInviteeRateBps(default, 1500) = %d, want 700", got)
	}
	if got := GetEffectiveSubscriptionReferralInviteeRateBps(setting, "vip", 800); got != 800 {
		t.Fatalf("GetEffectiveSubscriptionReferralInviteeRateBps(vip, 800) = %d, want 800", got)
	}
}

func TestGetEffectiveSubscriptionReferralTotalRateBpsUsesGroupedOverrideAndGroupDefaults(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalGlobalRate := common.SubscriptionReferralGlobalRateBps
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralGlobalRateBps = originalGlobalRate
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralGlobalRateBps = 1800
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"default":4500}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	user := seedReferralUser(t, db, "grouped-referral-user", 0, dto.UserSetting{})
	override, err := UpsertSubscriptionReferralOverride(user.Id, "default", 3200, 1)
	if err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}
	if override.Group != "default" {
		t.Fatalf("override.Group = %q, want %q", override.Group, "default")
	}

	if got := GetEffectiveSubscriptionReferralTotalRateBps(user.Id, "default"); got != 3200 {
		t.Fatalf("GetEffectiveSubscriptionReferralTotalRateBps(default) = %d, want 3200", got)
	}
	if got := GetEffectiveSubscriptionReferralTotalRateBps(user.Id, "vip"); got != 0 {
		t.Fatalf("GetEffectiveSubscriptionReferralTotalRateBps(vip) = %d, want 0", got)
	}

	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("clear UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	if got := GetEffectiveSubscriptionReferralTotalRateBps(user.Id, "vip"); got != 1800 {
		t.Fatalf("GetEffectiveSubscriptionReferralTotalRateBps(vip) after clear = %d, want 1800", got)
	}
}

func TestGetEffectiveSubscriptionReferralTotalRateBpsUsesNonDefaultGroupedOverride(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalGlobalRate := common.SubscriptionReferralGlobalRateBps
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralGlobalRateBps = originalGlobalRate
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralGlobalRateBps = 1800
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"default":4500,"vip":3000}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	user := seedReferralUser(t, db, "vip-grouped-referral-user", 0, dto.UserSetting{})
	override, err := UpsertSubscriptionReferralOverride(user.Id, "vip", 2200, 1)
	if err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}
	if override.Group != "vip" {
		t.Fatalf("override.Group = %q, want %q", override.Group, "vip")
	}

	loaded, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("GetSubscriptionReferralOverrideByUserIDAndGroup() error = %v", err)
	}
	if loaded.Group != "vip" {
		t.Fatalf("loaded.Group = %q, want %q", loaded.Group, "vip")
	}
	if loaded.TotalRateBps != 2200 {
		t.Fatalf("loaded.TotalRateBps = %d, want 2200", loaded.TotalRateBps)
	}

	if got := GetEffectiveSubscriptionReferralTotalRateBps(user.Id, "vip"); got != 2200 {
		t.Fatalf("GetEffectiveSubscriptionReferralTotalRateBps(vip) = %d, want 2200", got)
	}
}

func TestGetEffectiveSubscriptionReferralTotalRateBpsFallsBackToLegacyUngroupedOverride(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	originalGlobalRate := common.SubscriptionReferralGlobalRateBps
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		common.SubscriptionReferralGlobalRateBps = originalGlobalRate
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	common.SubscriptionReferralGlobalRateBps = 1800
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"default":4500,"vip":3000}`); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}

	user := seedReferralUser(t, db, "legacy-ungrouped-override-user", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "", 2600, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}

	if got := GetEffectiveSubscriptionReferralTotalRateBps(user.Id, "vip"); got != 2600 {
		t.Fatalf("GetEffectiveSubscriptionReferralTotalRateBps(vip) = %d, want 2600", got)
	}
}

func TestListSubscriptionReferralConfiguredGroupsIncludesPlanUpgradeGroups(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
		t.Fatalf("reset UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})

	planWithVIP := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, planWithVIP, "vip")
	planWithSpaces := seedReferralPlan(t, db, 29.9)
	setReferralPlanUpgradeGroup(t, db, planWithSpaces, " vip ")
	planDefault := seedReferralPlan(t, db, 39.9)
	setReferralPlanUpgradeGroup(t, db, planDefault, "")

	groups := ListSubscriptionReferralConfiguredGroups()
	if len(groups) != 1 || groups[0] != "vip" {
		t.Fatalf("ListSubscriptionReferralConfiguredGroups() = %v, want [vip]", groups)
	}
}

func TestIsSubscriptionReferralPlanBackedGroupRecognizesRetiredUpgradeGroup(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, plan, "retired")

	if !IsSubscriptionReferralPlanBackedGroup("retired") {
		t.Fatal("expected retired plan-backed group to be recognized")
	}
	if IsSubscriptionReferralPlanBackedGroup("ghost") {
		t.Fatal("expected unknown non-plan group to be rejected")
	}
}

func TestEnsureSubscriptionReferralOverrideSchemaReconcilesDuplicateGroupedRows(t *testing.T) {
	db := setupSubscriptionReferralOverrideSchemaDB(t)

	insertSQL := "INSERT INTO `subscription_referral_overrides` (`id`, `user_id`, `group`, `total_rate_bps`, `created_by`, `updated_by`, `created_at`, `updated_at`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	rows := []struct {
		id           int
		userID       int
		groupName    string
		totalRateBps int
		updatedAt    int64
	}{
		{id: 1, userID: 7, groupName: "vip", totalRateBps: 2200, updatedAt: 100},
		{id: 2, userID: 7, groupName: "vip", totalRateBps: 2400, updatedAt: 200},
		{id: 3, userID: 7, groupName: "vip", totalRateBps: 2600, updatedAt: 200},
		{id: 4, userID: 7, groupName: "", totalRateBps: 1800, updatedAt: 150},
	}
	for _, row := range rows {
		if err := db.Exec(insertSQL, row.id, row.userID, row.groupName, row.totalRateBps, 1, 1, row.updatedAt, row.updatedAt).Error; err != nil {
			t.Fatalf("failed to seed legacy override row %+v: %v", row, err)
		}
	}

	if err := ensureSubscriptionReferralOverrideSchema(); err != nil {
		t.Fatalf("ensureSubscriptionReferralOverrideSchema() error = %v", err)
	}

	var vipRows []SubscriptionReferralOverride
	if err := db.Where("user_id = ? AND `group` = ?", 7, "vip").Order("id ASC").Find(&vipRows).Error; err != nil {
		t.Fatalf("failed to load reconciled vip rows: %v", err)
	}
	if len(vipRows) != 1 {
		t.Fatalf("vip row count = %d, want 1", len(vipRows))
	}
	if vipRows[0].Id != 3 {
		t.Fatalf("reconciled vip row id = %d, want 3", vipRows[0].Id)
	}
	if vipRows[0].TotalRateBps != 2600 {
		t.Fatalf("reconciled vip row TotalRateBps = %d, want 2600", vipRows[0].TotalRateBps)
	}

	duplicate := &SubscriptionReferralOverride{
		UserId:       7,
		Group:        "vip",
		TotalRateBps: 3000,
		CreatedBy:    1,
		UpdatedBy:    1,
	}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("expected duplicate grouped row insert to fail after schema reconciliation")
	}
}

func TestSubscriptionReferralOverrideMigrationStartupPathHandlesDuplicateGroupedRows(t *testing.T) {
	db := setupSubscriptionReferralOverrideSchemaDB(t)

	insertSQL := "INSERT INTO `subscription_referral_overrides` (`id`, `user_id`, `group`, `total_rate_bps`, `created_by`, `updated_by`, `created_at`, `updated_at`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	rows := []struct {
		id           int
		userID       int
		groupName    string
		totalRateBps int
		updatedAt    int64
	}{
		{id: 11, userID: 9, groupName: "default", totalRateBps: 2000, updatedAt: 100},
		{id: 12, userID: 9, groupName: "default", totalRateBps: 2300, updatedAt: 200},
		{id: 13, userID: 9, groupName: "", totalRateBps: 1700, updatedAt: 150},
	}
	for _, row := range rows {
		if err := db.Exec(insertSQL, row.id, row.userID, row.groupName, row.totalRateBps, 1, 1, row.updatedAt, row.updatedAt).Error; err != nil {
			t.Fatalf("failed to seed legacy override row %+v: %v", row, err)
		}
	}

	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB() error = %v", err)
	}

	var defaultRows []SubscriptionReferralOverride
	if err := db.Where("user_id = ? AND `group` = ?", 9, "default").Order("id ASC").Find(&defaultRows).Error; err != nil {
		t.Fatalf("failed to load migrated default rows: %v", err)
	}
	if len(defaultRows) != 1 {
		t.Fatalf("default row count = %d, want 1", len(defaultRows))
	}
	if defaultRows[0].Id != 12 {
		t.Fatalf("retained default row id = %d, want 12", defaultRows[0].Id)
	}
	if defaultRows[0].TotalRateBps != 2300 {
		t.Fatalf("retained default row TotalRateBps = %d, want 2300", defaultRows[0].TotalRateBps)
	}

	duplicate := &SubscriptionReferralOverride{
		UserId:       9,
		Group:        "default",
		TotalRateBps: 2600,
		CreatedBy:    1,
		UpdatedBy:    1,
	}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("expected duplicate grouped row insert to fail after migrateDB() startup path")
	}
}

func TestSubscriptionReferralRecordSchemaMigrationPreservesLegacyRows(t *testing.T) {
	db := setupSubscriptionReferralRecordSchemaDB(t)

	insertSQL := "INSERT INTO `subscription_referral_records` (`id`, `order_id`, `order_trade_no`, `plan_id`, `payer_user_id`, `inviter_user_id`, `beneficiary_user_id`, `beneficiary_role`, `order_paid_amount`, `quota_per_unit_snapshot`, `total_rate_bps_snapshot`, `invitee_rate_bps_snapshot`, `applied_rate_bps`, `reward_quota`, `reversed_quota`, `debt_quota`, `status`, `created_at`, `updated_at`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	if err := db.Exec(insertSQL, 21, 301, "legacy-trade", 41, 51, 61, 71, SubscriptionReferralBeneficiaryRoleInviter, 12.5, 100.0, 4500, 500, 4000, 400, 10, 5, SubscriptionReferralStatusPartialRevert, int64(123), int64(456)).Error; err != nil {
		t.Fatalf("failed to seed legacy referral record row: %v", err)
	}

	if err := ensureSubscriptionReferralRecordSchema(); err != nil {
		t.Fatalf("ensureSubscriptionReferralRecordSchema() error = %v", err)
	}

	if !db.Migrator().HasColumn(&SubscriptionReferralRecord{}, "ReferralGroup") {
		t.Fatal("expected referral_group column to exist after schema migration")
	}
	if !db.Migrator().HasIndex(&SubscriptionReferralRecord{}, "ReferralGroup") {
		t.Fatal("expected referral_group index to exist after schema migration")
	}

	var record SubscriptionReferralRecord
	if err := db.Where("id = ?", 21).First(&record).Error; err != nil {
		t.Fatalf("failed to load migrated referral record row: %v", err)
	}
	if record.OrderTradeNo != "legacy-trade" {
		t.Fatalf("OrderTradeNo = %q, want legacy-trade", record.OrderTradeNo)
	}
	if record.RewardQuota != 400 || record.ReversedQuota != 10 || record.DebtQuota != 5 {
		t.Fatalf("unexpected quota fields after migration: %+v", record)
	}
	if record.ReferralGroup != "" {
		t.Fatalf("ReferralGroup = %q, want empty default", record.ReferralGroup)
	}
}

func TestUpsertSubscriptionReferralOverridePersistsPerGroup(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedReferralUser(t, db, "persist-per-group-user", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "default", 3500, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride(default) error = %v", err)
	}
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "vip", 2800, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride(vip) error = %v", err)
	}

	defaultOverride, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default")
	if err != nil {
		t.Fatalf("GetSubscriptionReferralOverrideByUserIDAndGroup(default) error = %v", err)
	}
	if defaultOverride.TotalRateBps != 3500 {
		t.Fatalf("default override TotalRateBps = %d, want 3500", defaultOverride.TotalRateBps)
	}

	vipOverride, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("GetSubscriptionReferralOverrideByUserIDAndGroup(vip) error = %v", err)
	}
	if vipOverride.TotalRateBps != 2800 {
		t.Fatalf("vip override TotalRateBps = %d, want 2800", vipOverride.TotalRateBps)
	}

	duplicate := &SubscriptionReferralOverride{
		UserId:       user.Id,
		Group:        "vip",
		TotalRateBps: 2700,
		CreatedBy:    1,
		UpdatedBy:    1,
	}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("expected duplicate (user_id, group) insert to fail")
	}
}

func TestDeleteSubscriptionReferralOverrideByUserIDAndGroupKeepsOtherGroups(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedReferralUser(t, db, "delete-per-group-user", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "default", 3500, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride(default) error = %v", err)
	}
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "vip", 2800, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride(vip) error = %v", err)
	}

	if err := DeleteSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default"); err != nil {
		t.Fatalf("DeleteSubscriptionReferralOverrideByUserIDAndGroup(default) error = %v", err)
	}

	if _, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetSubscriptionReferralOverrideByUserIDAndGroup(default) error = %v, want %v", err, gorm.ErrRecordNotFound)
	}

	vipOverride, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("GetSubscriptionReferralOverrideByUserIDAndGroup(vip) error = %v", err)
	}
	if vipOverride.TotalRateBps != 2800 {
		t.Fatalf("vip override TotalRateBps = %d, want 2800", vipOverride.TotalRateBps)
	}
}

func TestDeleteSubscriptionReferralOverrideByUserIDRemovesDefaultGroupedOverride(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedReferralUser(t, db, "delete-grouped-default-override", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "default", 3200, 1); err != nil {
		t.Fatalf("UpsertSubscriptionReferralOverride() error = %v", err)
	}

	if err := DeleteSubscriptionReferralOverrideByUserID(user.Id); err != nil {
		t.Fatalf("DeleteSubscriptionReferralOverrideByUserID() error = %v", err)
	}

	var count int64
	if err := db.Model(&SubscriptionReferralOverride{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("count override rows error = %v", err)
	}
	if count != 0 {
		t.Fatalf("override row count = %d, want 0", count)
	}
}
