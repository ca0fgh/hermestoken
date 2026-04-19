package model

import "testing"

func TestSubscriptionSettlementGroupSelectExprUsesConfiguredGroupQuoting(t *testing.T) {
	originalCommonGroupCol := commonGroupCol
	t.Cleanup(func() {
		commonGroupCol = originalCommonGroupCol
	})

	commonGroupCol = `"group"`
	if got, want := subscriptionSettlementGroupSelectExpr("batches"), `batches."group" AS settlement_group`; got != want {
		t.Fatalf("postgres select expr = %q, want %q", got, want)
	}

	commonGroupCol = "`group`"
	if got, want := subscriptionSettlementGroupSelectExpr("batches"), "batches.`group` AS settlement_group"; got != want {
		t.Fatalf("mysql select expr = %q, want %q", got, want)
	}
}
