package model

import (
	"errors"
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

	requiredIndexes := []struct {
		model interface{}
		name  string
	}{
		{&SubscriptionPlan{}, "UpgradeGroupKey"},
		{&PricingGroup{}, "GroupKey"},
		{&PricingGroupRatioOverride{}, "uk_pricing_group_ratio_overrides"},
		{&PricingGroupAutoPriority{}, "uk_pricing_group_auto_priorities_priority"},
	}
	for _, tc := range requiredIndexes {
		if !db.Migrator().HasIndex(tc.model, tc.name) {
			t.Fatalf("expected index %s on %T after migrateDB()", tc.name, tc.model)
		}
	}

	groupA := PricingGroup{GroupKey: "default", DisplayName: "Default"}
	if err := db.Create(&groupA).Error; err != nil {
		t.Fatalf("failed to seed source pricing group: %v", err)
	}
	groupB := PricingGroup{GroupKey: "premium", DisplayName: "Premium"}
	if err := db.Create(&groupB).Error; err != nil {
		t.Fatalf("failed to seed target pricing group: %v", err)
	}

	override := PricingGroupRatioOverride{SourceGroupId: groupA.Id, TargetGroupId: groupB.Id, Ratio: 1.25}
	if err := db.Create(&override).Error; err != nil {
		t.Fatalf("failed to seed pricing group ratio override: %v", err)
	}
	duplicateOverride := PricingGroupRatioOverride{SourceGroupId: groupA.Id, TargetGroupId: groupB.Id, Ratio: 1.5}
	if err := db.Create(&duplicateOverride).Error; err == nil {
		t.Fatal("expected duplicate pricing group ratio override to violate uniqueness")
	}

	priority := PricingGroupAutoPriority{GroupId: groupA.Id, Priority: 10}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("failed to seed pricing group auto priority: %v", err)
	}
	duplicatePriority := PricingGroupAutoPriority{GroupId: groupB.Id, Priority: 10}
	if err := db.Create(&duplicatePriority).Error; err == nil {
		t.Fatal("expected duplicate pricing group auto priority to violate uniqueness")
	}

	defaults := PricingGroup{}
	if err := db.Where("id = ?", groupA.Id).First(&defaults).Error; err != nil {
		t.Fatalf("failed to reload pricing group defaults: %v", err)
	}
	if defaults.Status != PricingGroupStatusActive {
		t.Fatalf("expected default pricing group status %d, got %d", PricingGroupStatusActive, defaults.Status)
	}

	rule := PricingGroupVisibilityRule{SubjectGroupId: groupA.Id, TargetGroupId: groupB.Id}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("failed to seed visibility rule: %v", err)
	}
	reloadedRule := PricingGroupVisibilityRule{}
	if err := db.Where("id = ?", rule.Id).First(&reloadedRule).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("failed to reload visibility rule defaults: %v", err)
	}
	if reloadedRule.Action != PricingGroupVisibilityActionAdd {
		t.Fatalf("expected default visibility rule action %q, got %q", PricingGroupVisibilityActionAdd, reloadedRule.Action)
	}
}
