package model

import (
	"fmt"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSubscriptionPlanSQLiteMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

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
	})

	return db
}

func subscriptionPlanSQLiteColumns(t *testing.T, db *gorm.DB) map[string]struct{} {
	t.Helper()

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(`subscription_plans`)").Scan(&cols).Error; err != nil {
		t.Fatalf("failed to inspect subscription_plans columns: %v", err)
	}

	result := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		result[col.Name] = struct{}{}
	}
	return result
}

func TestEnsureSubscriptionPlanTableSQLiteCreatesDeletedAtColumn(t *testing.T) {
	db := setupSubscriptionPlanSQLiteMigrationTestDB(t)

	if err := ensureSubscriptionPlanTableSQLite(); err != nil {
		t.Fatalf("ensureSubscriptionPlanTableSQLite() error = %v", err)
	}

	cols := subscriptionPlanSQLiteColumns(t, db)
	if _, ok := cols["deleted_at"]; !ok {
		t.Fatalf("expected deleted_at column to be created")
	}

	var count int64
	if err := db.Model(&SubscriptionPlan{}).Count(&count).Error; err != nil {
		t.Fatalf("expected SubscriptionPlan default query to succeed, got %v", err)
	}
}

func TestEnsureSubscriptionPlanTableSQLiteAddsDeletedAtColumnToLegacyTable(t *testing.T) {
	db := setupSubscriptionPlanSQLiteMigrationTestDB(t)

	legacyCreateSQL := `CREATE TABLE ` + "`subscription_plans`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`stock_total`" + ` integer DEFAULT 0,
` + "`stock_locked`" + ` integer DEFAULT 0,
` + "`stock_sold`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
	if err := db.Exec(legacyCreateSQL).Error; err != nil {
		t.Fatalf("failed to create legacy subscription_plans table: %v", err)
	}

	if err := ensureSubscriptionPlanTableSQLite(); err != nil {
		t.Fatalf("ensureSubscriptionPlanTableSQLite() error = %v", err)
	}

	cols := subscriptionPlanSQLiteColumns(t, db)
	if _, ok := cols["deleted_at"]; !ok {
		t.Fatalf("expected deleted_at column to be added to legacy table")
	}

	var count int64
	if err := db.Model(&SubscriptionPlan{}).Count(&count).Error; err != nil {
		t.Fatalf("expected SubscriptionPlan default query after legacy migration to succeed, got %v", err)
	}
}
