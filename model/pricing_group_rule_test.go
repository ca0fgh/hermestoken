package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPricingGroupRuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	dsn := filepath.Join(t.TempDir(), "pricing-group-rules.db")
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

	if err := db.AutoMigrate(
		&Option{},
		&PricingGroup{},
		&PricingGroupAlias{},
		&PricingGroupRatioOverride{},
		&PricingGroupVisibilityRule{},
		&PricingGroupAutoPriority{},
	); err != nil {
		t.Fatalf("failed to migrate pricing group rule tables: %v", err)
	}

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		InitColumnMetadata()
	})

	return db
}

func countPricingGroupRuleRows[T any](t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var count int64
	if err := db.Model(new(T)).Count(&count).Error; err != nil {
		t.Fatalf("failed to count rows for %T: %v", new(T), err)
	}
	return count
}

func TestSeedPricingGroupRulesFromLegacyOptions(t *testing.T) {
	db := setupPricingGroupRuleTestDB(t)

	if err := SeedPricingGroupsFromLegacyOptions(
		`{"default":1,"cc-opus4.6":1}`,
		`{"cc-opus4.6":"福利渠道"}`,
		`{"default":{"cc-opus4.6":0.8}}`,
		`["default","cc-opus4.6"]`,
		`{"default":{"+:cc-opus4.6":"福利渠道"}}`,
	); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	if count := countPricingGroupRuleRows[PricingGroup](t, db); count != 2 {
		t.Fatalf("expected 2 pricing groups, got %d", count)
	}
	if count := countPricingGroupRuleRows[PricingGroupRatioOverride](t, db); count != 1 {
		t.Fatalf("expected 1 pricing group ratio override, got %d", count)
	}
	if count := countPricingGroupRuleRows[PricingGroupVisibilityRule](t, db); count != 1 {
		t.Fatalf("expected 1 pricing group visibility rule, got %d", count)
	}
	if count := countPricingGroupRuleRows[PricingGroupAutoPriority](t, db); count != 2 {
		t.Fatalf("expected 2 auto priorities, got %d", count)
	}

	var premium PricingGroup
	if err := db.Where("group_key = ?", "cc-opus4.6").First(&premium).Error; err != nil {
		t.Fatalf("failed to load seeded pricing group: %v", err)
	}
	if premium.DisplayName != "福利渠道" {
		t.Fatalf("expected display name 福利渠道, got %q", premium.DisplayName)
	}
	if !premium.UserSelectable {
		t.Fatalf("expected seeded premium group to be user selectable")
	}

	var override PricingGroupRatioOverride
	if err := db.First(&override).Error; err != nil {
		t.Fatalf("failed to load ratio override: %v", err)
	}
	if override.Ratio != 0.8 {
		t.Fatalf("expected ratio override 0.8, got %v", override.Ratio)
	}

	var rule PricingGroupVisibilityRule
	if err := db.First(&rule).Error; err != nil {
		t.Fatalf("failed to load visibility rule: %v", err)
	}
	if rule.Action != PricingGroupVisibilityActionAdd {
		t.Fatalf("expected visibility action add, got %q", rule.Action)
	}
	if rule.DescriptionOverride != "福利渠道" {
		t.Fatalf("expected visibility description override 福利渠道, got %q", rule.DescriptionOverride)
	}
}

func TestUpdateOptionSyncsCanonicalPricingGroupRules(t *testing.T) {
	db := setupPricingGroupRuleTestDB(t)

	if err := UpdateOption("GroupRatio", `{"default":1,"premium":1}`); err != nil {
		t.Fatalf("failed to update group ratio option: %v", err)
	}
	if err := UpdateOption("UserUsableGroups", `{"premium":"Premium"}`); err != nil {
		t.Fatalf("failed to update usable groups option: %v", err)
	}
	if err := UpdateOption("AutoGroups", `["premium"]`); err != nil {
		t.Fatalf("failed to update auto groups option: %v", err)
	}
	if err := UpdateOption("group_ratio_setting.group_special_usable_group", `{"default":{"+:premium":"Premium"}}`); err != nil {
		t.Fatalf("failed to update special usable groups option: %v", err)
	}

	if count := countPricingGroupRuleRows[PricingGroupAutoPriority](t, db); count != 1 {
		t.Fatalf("expected canonical auto group priorities to be synced from options, got %d", count)
	}
	if count := countPricingGroupRuleRows[PricingGroupVisibilityRule](t, db); count != 1 {
		t.Fatalf("expected canonical visibility rules to be synced from options, got %d", count)
	}

	var premium PricingGroup
	if err := db.Where("group_key = ?", "premium").First(&premium).Error; err != nil {
		t.Fatalf("expected synced premium pricing group, got error: %v", err)
	}
	if premium.DisplayName != "Premium" {
		t.Fatalf("expected synced premium display name Premium, got %q", premium.DisplayName)
	}
	if !premium.UserSelectable {
		t.Fatalf("expected synced premium pricing group to be user selectable")
	}
}
