package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

func TestGetUserWithdrawalSettingNormalizesMinAmountToMatchRule(t *testing.T) {
	common.OptionMap = map[string]string{
		WithdrawalEnabledOptionKey:     "true",
		WithdrawalMinAmountOptionKey:   "10",
		WithdrawalInstructionOptionKey: "manual payout",
		WithdrawalFeeRulesOptionKey:    `[{"min_amount":10,"max_amount":0,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`,
	}

	setting := GetUserWithdrawalSetting()
	if setting.MinAmount != 10.01 {
		t.Fatalf("GetUserWithdrawalSetting min amount = %.2f, want 10.01", setting.MinAmount)
	}
}

func TestCreateUserWithdrawalFreezesQuotaAndStoresSnapshots(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-model-user", 100000)
	applyQuota := int(decimal.NewFromFloat(100).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart())
	common.OptionMap = map[string]string{
		WithdrawalEnabledOptionKey:     "true",
		WithdrawalMinAmountOptionKey:   "10",
		WithdrawalInstructionOptionKey: "manual payout",
		WithdrawalFeeRulesOptionKey:    `[{"min_amount":10,"max_amount":0,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`,
	}

	order, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         100,
		AlipayAccount:  "alice@example.com",
		AlipayRealName: "Alice",
	})
	if err != nil {
		t.Fatalf("CreateUserWithdrawal returned error: %v", err)
	}

	refreshed, _ := GetUserById(user.Id, true)
	if refreshed.Quota != 100000-applyQuota {
		t.Fatalf("quota = %d, want %d", refreshed.Quota, 100000-applyQuota)
	}
	if refreshed.WithdrawFrozenQuota != applyQuota {
		t.Fatalf("withdraw_frozen_quota = %d, want %d", refreshed.WithdrawFrozenQuota, applyQuota)
	}
	if order.Status != UserWithdrawalStatusPending {
		t.Fatalf("status = %s, want pending", order.Status)
	}
	if order.FeeAmount != 2 || order.NetAmount != 98 {
		t.Fatalf("fee/net = %v/%v, want 2/98", order.FeeAmount, order.NetAmount)
	}
	if order.TradeNo == "" {
		t.Fatal("expected trade no to be generated")
	}
}

func TestRejectApprovedWithdrawalReturnsFrozenQuota(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-approved-user", 100000)
	withdrawal := seedApprovedWithdrawal(t, db, user.Id, 100)

	if err := RejectUserWithdrawal(withdrawal.Id, 99, "manual reject"); err != nil {
		t.Fatalf("RejectUserWithdrawal returned error: %v", err)
	}

	refreshed, _ := GetUserById(user.Id, true)
	if refreshed.Quota != 100000 {
		t.Fatalf("quota = %d, want restored 100000", refreshed.Quota)
	}
	if refreshed.WithdrawFrozenQuota != 0 {
		t.Fatalf("withdraw_frozen_quota = %d, want 0", refreshed.WithdrawFrozenQuota)
	}
}

func TestCreateUserWithdrawalRejectsSecondOpenOrder(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-open-order-user", 100000)
	seedPendingWithdrawal(t, db, user.Id, 100)
	common.OptionMap = map[string]string{
		WithdrawalEnabledOptionKey:     "true",
		WithdrawalMinAmountOptionKey:   "10",
		WithdrawalInstructionOptionKey: "manual payout",
		WithdrawalFeeRulesOptionKey:    `[{"min_amount":0,"max_amount":100,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`,
	}

	_, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         50,
		AlipayAccount:  "alice@example.com",
		AlipayRealName: "Alice",
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "still pending") {
		t.Fatalf("CreateUserWithdrawal error = %v, want open-order validation", err)
	}
}

func TestCreateUserWithdrawalRejectsAmountWithoutMatchingFeeRule(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-gap-user", 100000)
	common.OptionMap = map[string]string{
		WithdrawalEnabledOptionKey:     "true",
		WithdrawalMinAmountOptionKey:   "10",
		WithdrawalInstructionOptionKey: "manual payout",
		WithdrawalFeeRulesOptionKey:    `[{"min_amount":0,"max_amount":100,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`,
	}

	_, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         150,
		AlipayAccount:  "alice@example.com",
		AlipayRealName: "Alice",
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "fee rule") {
		t.Fatalf("CreateUserWithdrawal error = %v, want missing fee rule validation", err)
	}
}

func TestMarkPaidConsumesFrozenQuotaWithoutTouchingAvailableQuota(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-paid-user", 99900)
	withdrawal := seedApprovedWithdrawal(t, db, user.Id, 100)
	expectedQuota := user.Quota - withdrawal.ApplyQuota

	if err := MarkUserWithdrawalPaid(withdrawal.Id, 88, MarkUserWithdrawalPaidParams{PayReceiptNo: "ALI123"}); err != nil {
		t.Fatalf("MarkUserWithdrawalPaid returned error: %v", err)
	}

	refreshed, _ := GetUserById(user.Id, true)
	if refreshed.Quota != expectedQuota {
		t.Fatalf("quota = %d, want unchanged %d", refreshed.Quota, expectedQuota)
	}
	if refreshed.WithdrawFrozenQuota != 0 {
		t.Fatalf("withdraw_frozen_quota = %d, want 0", refreshed.WithdrawFrozenQuota)
	}
}

func TestParseWithdrawalFeeRulesRejectsOverlappingRanges(t *testing.T) {
	_, err := ParseWithdrawalFeeRules(`[{"min_amount":10,"max_amount":100,"fee_type":"fixed","fee_value":1,"enabled":true,"sort_order":1},{"min_amount":99,"max_amount":0,"fee_type":"fixed","fee_value":1,"enabled":true,"sort_order":2}]`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "overlap") {
		t.Fatalf("ParseWithdrawalFeeRules error = %v, want overlap validation", err)
	}
}

func TestCalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges(t *testing.T) {
	rules, err := ParseWithdrawalFeeRules(`[{"min_amount":0,"max_amount":100,"fee_type":"fixed","fee_value":5,"enabled":true,"sort_order":1},{"min_amount":100,"max_amount":500,"fee_type":"ratio","fee_value":3,"enabled":true,"sort_order":2}]`)
	if err != nil {
		t.Fatalf("ParseWithdrawalFeeRules returned error: %v", err)
	}

	matchedRule, feeAmount, err := calculateWithdrawalFeeAmount(decimal.NewFromInt(100), rules)
	if err != nil {
		t.Fatalf("calculateWithdrawalFeeAmount returned error: %v", err)
	}
	if matchedRule == nil {
		t.Fatal("expected first rule to match amount 100")
	}
	if matchedRule.SortOrder != 1 {
		t.Fatalf("matched rule sort order = %d, want 1", matchedRule.SortOrder)
	}
	if !feeAmount.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("fee amount = %s, want 5", feeAmount.String())
	}
}

func TestParseWithdrawalFeeRulesRejectsEmptyRanges(t *testing.T) {
	_, err := ParseWithdrawalFeeRules(`[{"min_amount":100,"max_amount":100,"fee_type":"fixed","fee_value":1,"enabled":true,"sort_order":1}]`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "range") {
		t.Fatalf("ParseWithdrawalFeeRules error = %v, want range validation", err)
	}
}
