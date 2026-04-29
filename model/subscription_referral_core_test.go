package model

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

func TestNormalizeSubscriptionReferralRateBps(t *testing.T) {
	testCases := []struct {
		input int
		want  int
	}{
		{input: -1, want: 0},
		{input: 0, want: 0},
		{input: 500, want: 500},
		{input: 10000, want: 10000},
		{input: 12000, want: 10000},
	}

	for _, tc := range testCases {
		if got := NormalizeSubscriptionReferralRateBps(tc.input); got != tc.want {
			t.Fatalf("NormalizeSubscriptionReferralRateBps(%d) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestResolveSubscriptionReferralConfigClampsInviteeRate(t *testing.T) {
	cfg := ResolveSubscriptionReferralConfig(3500, 5000)
	if !cfg.Enabled {
		t.Fatal("expected config to be enabled")
	}
	if cfg.TotalRateBps != 3500 {
		t.Fatalf("TotalRateBps = %d, want 3500", cfg.TotalRateBps)
	}
	if cfg.InviteeRateBps != 3500 {
		t.Fatalf("InviteeRateBps = %d, want 3500", cfg.InviteeRateBps)
	}
	if cfg.InviterRateBps != 0 {
		t.Fatalf("InviterRateBps = %d, want 0", cfg.InviterRateBps)
	}
}

func TestCalculateSubscriptionReferralQuotaUsesMoneyAndQuotaPerUnit(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	got := CalculateSubscriptionReferralQuota(10, 2000)
	if got != 200 {
		t.Fatalf("CalculateSubscriptionReferralQuota() = %d, want 200", got)
	}
}

func TestReverseSubscriptionReferralByTradeNoRejectsUnknownTradeNo(t *testing.T) {
	setupReferralTemplateDB(t)

	if err := ReverseSubscriptionReferralByTradeNo("missing-trade-no", 1); err == nil {
		t.Fatal("expected missing trade number to fail")
	}
}
