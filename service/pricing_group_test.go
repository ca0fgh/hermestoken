package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func resetPricingGroupTestTables(t *testing.T) {
	t.Helper()
	if err := model.DB.Exec("DELETE FROM pricing_group_auto_priorities").Error; err != nil {
		t.Fatalf("failed to clear pricing_group_auto_priorities: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM pricing_group_visibility_rules").Error; err != nil {
		t.Fatalf("failed to clear pricing_group_visibility_rules: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM pricing_group_ratio_overrides").Error; err != nil {
		t.Fatalf("failed to clear pricing_group_ratio_overrides: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM pricing_group_aliases").Error; err != nil {
		t.Fatalf("failed to clear pricing_group_aliases: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM pricing_groups").Error; err != nil {
		t.Fatalf("failed to clear pricing_groups: %v", err)
	}
	t.Cleanup(func() {
		if err := model.DB.Exec("DELETE FROM pricing_group_auto_priorities").Error; err != nil {
			t.Fatalf("failed to cleanup pricing_group_auto_priorities: %v", err)
		}
		if err := model.DB.Exec("DELETE FROM pricing_group_visibility_rules").Error; err != nil {
			t.Fatalf("failed to cleanup pricing_group_visibility_rules: %v", err)
		}
		if err := model.DB.Exec("DELETE FROM pricing_group_ratio_overrides").Error; err != nil {
			t.Fatalf("failed to cleanup pricing_group_ratio_overrides: %v", err)
		}
		if err := model.DB.Exec("DELETE FROM pricing_group_aliases").Error; err != nil {
			t.Fatalf("failed to cleanup pricing_group_aliases: %v", err)
		}
		if err := model.DB.Exec("DELETE FROM pricing_groups").Error; err != nil {
			t.Fatalf("failed to cleanup pricing_groups: %v", err)
		}
	})
}

func seedPricingGroup(t *testing.T, groupKey string) model.PricingGroup {
	t.Helper()
	group := model.PricingGroup{
		GroupKey:       groupKey,
		DisplayName:    groupKey,
		BillingRatio:   1,
		UserSelectable: true,
		Status:         model.PricingGroupStatusActive,
	}
	if err := model.DB.Create(&group).Error; err != nil {
		t.Fatalf("failed to create pricing group %q: %v", groupKey, err)
	}
	return group
}

func seedPricingGroupAlias(t *testing.T, aliasKey string, groupID int) {
	t.Helper()
	alias := model.PricingGroupAlias{AliasKey: aliasKey, GroupId: groupID, Reason: "test"}
	if err := model.DB.Create(&alias).Error; err != nil {
		t.Fatalf("failed to create pricing group alias %q: %v", aliasKey, err)
	}
}

func withPricingGroupLegacyRatios(t *testing.T, ratioJSON string) {
	t.Helper()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}
	t.Cleanup(func() {
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func withPricingGroupAutoGroups(t *testing.T, autoGroupsJSON string) {
	t.Helper()
	originalAutoGroups := setting.AutoGroups2JsonString()
	if err := setting.UpdateAutoGroupsByJsonString(autoGroupsJSON); err != nil {
		t.Fatalf("failed to set auto groups: %v", err)
	}
	t.Cleanup(func() {
		if err := setting.UpdateAutoGroupsByJsonString(originalAutoGroups); err != nil {
			t.Fatalf("failed to restore auto groups: %v", err)
		}
	})
}

func TestResolveCanonicalPricingGroupKey(t *testing.T) {
	resetPricingGroupTestTables(t)
	canonical := seedPricingGroup(t, "default")
	seedPricingGroupAlias(t, "legacy-default", canonical.Id)

	t.Run("resolves exact canonical key", func(t *testing.T) {
		resolved, err := ResolveCanonicalPricingGroupKey("  default  ")
		if err != nil {
			t.Fatalf("expected canonical key resolution to succeed, got %v", err)
		}
		if resolved.CanonicalKey != "default" {
			t.Fatalf("expected canonical key default, got %q", resolved.CanonicalKey)
		}
		if resolved.Source != PricingGroupResolutionSourceCanonical {
			t.Fatalf("expected canonical source, got %q", resolved.Source)
		}
	})

	t.Run("resolves alias key", func(t *testing.T) {
		resolved, err := ResolveCanonicalPricingGroupKey(" legacy-default ")
		if err != nil {
			t.Fatalf("expected alias resolution to succeed, got %v", err)
		}
		if resolved.CanonicalKey != "default" {
			t.Fatalf("expected alias to resolve to default, got %q", resolved.CanonicalKey)
		}
		if resolved.Source != PricingGroupResolutionSourceAlias {
			t.Fatalf("expected alias source, got %q", resolved.Source)
		}
	})
}

func TestResolveCanonicalPricingGroupKeyRejectsUnknownValue(t *testing.T) {
	resetPricingGroupTestTables(t)
	seedPricingGroup(t, "default")

	resolved, err := ResolveCanonicalPricingGroupKey("missing-group")
	if err == nil {
		t.Fatal("expected unknown pricing group to be rejected")
	}
	if resolved.CanonicalKey != "" {
		t.Fatalf("expected empty canonical key for unknown group, got %q", resolved.CanonicalKey)
	}
	if resolved.Source != PricingGroupResolutionSourceUnknown {
		t.Fatalf("expected unknown source, got %q", resolved.Source)
	}
}

func TestListCanonicalPricingGroupKeysOrFallbackUsesLegacyOnlyBeforeSeeding(t *testing.T) {
	resetPricingGroupTestTables(t)
	withPricingGroupLegacyRatios(t, `{"default":1,"legacy-only":1}`)

	beforeSeeding, err := ListCanonicalPricingGroupKeysOrFallback()
	if err != nil {
		t.Fatalf("expected pre-seeding fallback to succeed, got %v", err)
	}
	if len(beforeSeeding) != 2 || beforeSeeding[0] != "default" || beforeSeeding[1] != "legacy-only" {
		t.Fatalf("expected pre-seeding fallback to use legacy keys, got %#v", beforeSeeding)
	}

	seedPricingGroup(t, "default")
	afterSeeding, err := ListCanonicalPricingGroupKeysOrFallback()
	if err != nil {
		t.Fatalf("expected canonical listing to succeed after seeding, got %v", err)
	}
	if len(afterSeeding) != 1 || afterSeeding[0] != "default" {
		t.Fatalf("expected seeded canonical keys to suppress legacy fallback, got %#v", afterSeeding)
	}
}

func TestResolveCanonicalPricingGroupKeyReturnsOperationalErrorWhenStoreUnavailable(t *testing.T) {
	originalDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = originalDB
	})

	resolved, err := ResolveCanonicalPricingGroupKey("default")
	if err == nil {
		t.Fatal("expected unavailable store to return an operational error")
	}
	if resolved.Source == PricingGroupResolutionSourceUnknown || resolved.Source == PricingGroupResolutionSourceEmpty {
		t.Fatalf("expected operational error to avoid business miss sources, got %q", resolved.Source)
	}
}

func TestBuildPricingGroupConsistencyReportIncludesAutoGroups(t *testing.T) {
	resetPricingGroupTestTables(t)
	withPricingGroupLegacyRatios(t, `{"default":1,"legacy-default":1,"legacy-missing":1}`)
	withPricingGroupAutoGroups(t, `["default","legacy-default","auto-missing"]`)

	group := seedPricingGroup(t, "default")
	seedPricingGroupAlias(t, "legacy-default", group.Id)

	report, err := BuildPricingGroupConsistencyReport()
	if err != nil {
		t.Fatalf("expected consistency report build to succeed, got %v", err)
	}

	reported := make(map[string]string, len(report.UnresolvedLegacyReferences))
	for _, unresolved := range report.UnresolvedLegacyReferences {
		reported[unresolved.Scope+"|"+unresolved.Value] = unresolved.Value
	}

	if _, ok := reported["auto_groups|auto-missing"]; !ok {
		t.Fatalf("expected stale auto group to be reported, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["group_ratio|legacy-default"]; ok {
		t.Fatalf("expected alias-backed legacy-default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["auto_groups|legacy-default"]; ok {
		t.Fatalf("expected alias-backed auto group to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
}

func TestGetUserSelectableGroupsForUserUsesCanonicalPricingGroupsBeforeLegacyFallback(t *testing.T) {
	resetPricingGroupTestTables(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"legacy-only":"旧分组"}`,
		`{}`,
		`{"legacy-only":1}`,
	)
	withPricingGroupAutoGroups(t, `["legacy-only"]`)

	if err := model.SeedPricingGroupsFromLegacyOptions(
		`{"default":1,"premium":1}`,
		`{"premium":"Premium"}`,
		`{}`,
		`["premium"]`,
		`{}`,
	); err != nil {
		t.Fatalf("failed to seed canonical pricing groups: %v", err)
	}

	groups := GetUserSelectableGroupsForUser(0, "default")
	if got := groups["premium"]; got != "Premium" {
		t.Fatalf("expected canonical premium selectable group, got %q from %#v", got, groups)
	}
	if _, ok := groups["legacy-only"]; ok {
		t.Fatalf("expected stale legacy selectable group to be ignored once canonical data exists, got %#v", groups)
	}

	autoGroups := GetUserAutoGroupForUser(0, "default")
	if len(autoGroups) != 1 || autoGroups[0] != "premium" {
		t.Fatalf("expected canonical auto groups [premium], got %#v", autoGroups)
	}
}
