package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateAddsPricingGroupTablesAndColumns(t *testing.T) {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	dsn := filepath.Join(t.TempDir(), "pricing-group-migrate.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitColumnMetadata()

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		InitColumnMetadata()
	})

	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB() error = %v", err)
	}

	requiredTables := []string{
		"pricing_groups",
		"pricing_group_aliases",
		"pricing_group_ratio_overrides",
		"pricing_group_visibility_rules",
		"pricing_group_auto_priorities",
	}
	for _, tableName := range requiredTables {
		if !db.Migrator().HasTable(tableName) {
			t.Fatalf("expected table %s to exist after migrateDB()", tableName)
		}
	}

	requiredColumns := []struct {
		model  interface{}
		column string
	}{
		{&User{}, "GroupKey"},
		{&Token{}, "SelectionMode"},
		{&Token{}, "GroupKey"},
		{&SubscriptionPlan{}, "UpgradeGroupKey"},
		{&UserSubscription{}, "UpgradeGroupKeySnapshot"},
		{&UserSubscription{}, "UpgradeGroupNameSnapshot"},
	}
	for _, tc := range requiredColumns {
		if !db.Migrator().HasColumn(tc.model, tc.column) {
			t.Fatalf("expected column %T.%s to exist after migrateDB()", tc.model, tc.column)
		}
	}
}
