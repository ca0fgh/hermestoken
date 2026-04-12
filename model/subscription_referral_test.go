package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

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
