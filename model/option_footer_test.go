package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUpdateOptionMapPreservesExplicitEmptyFooterValue(t *testing.T) {
	originalFooter := common.Footer
	originalMap := common.OptionMap
	defer func() {
		common.Footer = originalFooter
		common.OptionMap = originalMap
	}()

	common.Footer = ""
	common.OptionMap = make(map[string]string)

	if err := updateOptionMap("Footer", ""); err != nil {
		t.Fatalf("updateOptionMap returned error: %v", err)
	}

	if common.Footer != "" {
		t.Fatalf("common.Footer = %q, want empty string", common.Footer)
	}
	if common.OptionMap["Footer"] != "" {
		t.Fatalf("OptionMap[Footer] = %q, want empty string", common.OptionMap["Footer"])
	}
}

func TestEnsureDefaultOptionRecordCreatesMissingFooterRow(t *testing.T) {
	originalDB := DB
	defer func() {
		DB = originalDB
	}()

	tempDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open temp db: %v", err)
	}
	if err := tempDB.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("failed to migrate options table: %v", err)
	}
	DB = tempDB

	if err := ensureDefaultOptionRecord("Footer", common.DefaultFooterHTML); err != nil {
		t.Fatalf("ensureDefaultOptionRecord returned error: %v", err)
	}

	var option Option
	if err := DB.First(&option, "key = ?", "Footer").Error; err != nil {
		t.Fatalf("failed to load persisted footer option: %v", err)
	}
	if option.Value != common.DefaultFooterHTML {
		t.Fatalf("persisted Footer = %q, want default footer html", option.Value)
	}
}

func TestLegacySubscriptionReferralOptionsAreNotExposedThroughOptionMap(t *testing.T) {
	originalEnabled := common.SubscriptionReferralEnabled
	originalGlobalRateBps := common.SubscriptionReferralGlobalRateBps
	originalGroupRatesJSON := common.SubscriptionReferralGroupRates2JSONString()
	originalMap := common.OptionMap
	defer func() {
		common.SubscriptionReferralEnabled = originalEnabled
		common.SubscriptionReferralGlobalRateBps = originalGlobalRateBps
		common.OptionMap = originalMap
		_ = common.UpdateSubscriptionReferralGroupRatesByJSONString(originalGroupRatesJSON)
	}()

	common.OptionMap = make(map[string]string)

	if err := updateOptionMap("SubscriptionReferralEnabled", "true"); err != nil {
		t.Fatalf("updateOptionMap(enabled) returned error: %v", err)
	}
	if _, exists := common.OptionMap["SubscriptionReferralEnabled"]; exists {
		t.Fatal("legacy SubscriptionReferralEnabled should not be exposed through OptionMap")
	}

	if err := updateOptionMap("SubscriptionReferralGlobalRateBps", "4500"); err != nil {
		t.Fatalf("updateOptionMap(global_rate) returned error: %v", err)
	}
	if _, exists := common.OptionMap["SubscriptionReferralGlobalRateBps"]; exists {
		t.Fatal("legacy SubscriptionReferralGlobalRateBps should not be exposed through OptionMap")
	}

	if err := updateOptionMap("SubscriptionReferralGroupRates", `{"vip":4500}`); err != nil {
		t.Fatalf("updateOptionMap(group_rates) returned error: %v", err)
	}
	if _, exists := common.OptionMap["SubscriptionReferralGroupRates"]; exists {
		t.Fatal("legacy SubscriptionReferralGroupRates should not be exposed through OptionMap")
	}
}
