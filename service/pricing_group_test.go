package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func resetPricingGroupTestTables(t *testing.T) {
	t.Helper()
	if err := model.DB.Exec("DELETE FROM pricing_group_aliases").Error; err != nil {
		t.Fatalf("failed to clear pricing_group_aliases: %v", err)
	}
	if err := model.DB.Exec("DELETE FROM pricing_groups").Error; err != nil {
		t.Fatalf("failed to clear pricing_groups: %v", err)
	}
	t.Cleanup(func() {
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
	group := model.PricingGroup{GroupKey: groupKey, DisplayName: groupKey}
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
