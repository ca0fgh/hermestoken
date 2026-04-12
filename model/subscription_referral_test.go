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

	cfg := ResolveSubscriptionReferralConfig(3500, setting)
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

	cfg = ResolveSubscriptionReferralConfig(1800, setting)
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
