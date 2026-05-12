package model

import (
	"encoding/json"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
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

func TestEnsureAnthropicPricingCorrectionsFixesKnownStaleHaikuValues(t *testing.T) {
	originalDB := DB
	originalMap := common.OptionMap
	originalModelRatio := ratio_setting.ModelRatio2JSONString()
	originalCompletionRatio := ratio_setting.CompletionRatio2JSONString()
	defer func() {
		DB = originalDB
		common.OptionMap = originalMap
		_ = ratio_setting.UpdateModelRatioByJSONString(originalModelRatio)
		_ = ratio_setting.UpdateCompletionRatioByJSONString(originalCompletionRatio)
	}()

	tempDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open temp db: %v", err)
	}
	if err := tempDB.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("failed to migrate options table: %v", err)
	}
	DB = tempDB
	common.OptionMap = map[string]string{}

	if err := tempDB.Create(&Option{
		Key:   "ModelRatio",
		Value: `{"claude-haiku-4-5-20251001":0.07,"manual-model":0.2}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed model ratio option: %v", err)
	}
	if err := tempDB.Create(&Option{
		Key:   "CompletionRatio",
		Value: `{"claude-haiku-4-5-20251001":5.071429,"manual-model":3}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed completion ratio option: %v", err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{"claude-haiku-4-5-20251001":0.07,"manual-model":0.2}`); err != nil {
		t.Fatalf("failed to seed in-memory model ratio: %v", err)
	}
	if err := ratio_setting.UpdateCompletionRatioByJSONString(`{"claude-haiku-4-5-20251001":5.071429,"manual-model":3}`); err != nil {
		t.Fatalf("failed to seed in-memory completion ratio: %v", err)
	}

	if err := ensureAnthropicPricingCorrections(); err != nil {
		t.Fatalf("ensureAnthropicPricingCorrections returned error: %v", err)
	}

	var modelRatio Option
	if err := tempDB.First(&modelRatio, "key = ?", "ModelRatio").Error; err != nil {
		t.Fatalf("failed to load model ratio option: %v", err)
	}
	var modelRatios map[string]float64
	if err := json.Unmarshal([]byte(modelRatio.Value), &modelRatios); err != nil {
		t.Fatalf("failed to decode model ratio option: %v", err)
	}
	if modelRatios["claude-haiku-4-5-20251001"] != 0.5 {
		t.Fatalf("haiku model ratio = %v, want 0.5", modelRatios["claude-haiku-4-5-20251001"])
	}
	if modelRatios["manual-model"] != 0.2 {
		t.Fatalf("manual model ratio = %v, want 0.2", modelRatios["manual-model"])
	}

	var completionRatio Option
	if err := tempDB.First(&completionRatio, "key = ?", "CompletionRatio").Error; err != nil {
		t.Fatalf("failed to load completion ratio option: %v", err)
	}
	var completionRatios map[string]float64
	if err := json.Unmarshal([]byte(completionRatio.Value), &completionRatios); err != nil {
		t.Fatalf("failed to decode completion ratio option: %v", err)
	}
	if completionRatios["claude-haiku-4-5-20251001"] != 5 {
		t.Fatalf("haiku completion ratio = %v, want 5", completionRatios["claude-haiku-4-5-20251001"])
	}
	if completionRatios["manual-model"] != 3 {
		t.Fatalf("manual completion ratio = %v, want 3", completionRatios["manual-model"])
	}
}

func TestUpdateOptionRejectsInvalidMarketplaceFeeRateBeforePersisting(t *testing.T) {
	originalDB := DB
	originalMap := common.OptionMap
	originalFeeRate := setting.MarketplaceFeeRate
	defer func() {
		DB = originalDB
		common.OptionMap = originalMap
		setting.MarketplaceFeeRate = originalFeeRate
	}()

	tempDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open temp db: %v", err)
	}
	if err := tempDB.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("failed to migrate options table: %v", err)
	}
	DB = tempDB
	common.OptionMap = map[string]string{"MarketplaceFeeRate": "0.05"}
	setting.MarketplaceFeeRate = 0.05

	if err := UpdateOption("MarketplaceFeeRate", "-0.01"); err == nil {
		t.Fatal("expected invalid marketplace fee rate to fail")
	}

	var count int64
	if err := DB.Model(&Option{}).Where("key = ?", "MarketplaceFeeRate").Count(&count).Error; err != nil {
		t.Fatalf("failed to count marketplace fee option: %v", err)
	}
	if count != 0 {
		t.Fatalf("MarketplaceFeeRate option rows = %d, want 0", count)
	}
	if common.OptionMap["MarketplaceFeeRate"] != "0.05" {
		t.Fatalf("OptionMap fee rate = %q, want 0.05", common.OptionMap["MarketplaceFeeRate"])
	}
	if setting.MarketplaceFeeRate != 0.05 {
		t.Fatalf("setting.MarketplaceFeeRate = %v, want 0.05", setting.MarketplaceFeeRate)
	}
}

func TestUpdateOptionMapRejectsInvalidMarketplaceFeeRateWithoutChangingMemory(t *testing.T) {
	originalMap := common.OptionMap
	originalFeeRate := setting.MarketplaceFeeRate
	defer func() {
		common.OptionMap = originalMap
		setting.MarketplaceFeeRate = originalFeeRate
	}()

	common.OptionMap = map[string]string{"MarketplaceFeeRate": "0.05"}
	setting.MarketplaceFeeRate = 0.05

	if err := updateOptionMap("MarketplaceFeeRate", "NaN"); err == nil {
		t.Fatal("expected invalid marketplace fee rate to fail")
	}
	if common.OptionMap["MarketplaceFeeRate"] != "0.05" {
		t.Fatalf("OptionMap fee rate = %q, want 0.05", common.OptionMap["MarketplaceFeeRate"])
	}
	if setting.MarketplaceFeeRate != 0.05 {
		t.Fatalf("setting.MarketplaceFeeRate = %v, want 0.05", setting.MarketplaceFeeRate)
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

func TestLegacyChannelAutomationOptionsAreNotExposedThroughOptionMap(t *testing.T) {
	originalMap := common.OptionMap
	defer func() {
		common.OptionMap = originalMap
	}()

	common.OptionMap = make(map[string]string)

	for _, key := range []string{
		"AutomaticDisableChannelEnabled",
		"AutomaticEnableChannelEnabled",
		"ChannelDisableThreshold",
		"AutomaticDisableKeywords",
		"AutomaticDisableStatusCodes",
		"monitor_setting.auto_disabled_channel_recovery_cooldown_minutes",
		"monitor_setting.auto_test_channel_enabled",
		"monitor_setting.auto_test_channel_minutes",
	} {
		if err := updateOptionMap(key, "true"); err != nil {
			t.Fatalf("updateOptionMap(%s) returned error: %v", key, err)
		}
		if _, exists := common.OptionMap[key]; exists {
			t.Fatalf("legacy channel automation option %q should not be exposed through OptionMap", key)
		}
	}
}
