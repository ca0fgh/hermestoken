package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestGetSubscriptionPlanInfoByUserSubscriptionIdReturnsDeletedPlanTitle(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 12.5)

	subscription := &UserSubscription{
		UserId:      1,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		StartTime:   1,
		EndTime:     common.GetTimestamp() + 3600,
		Status:      "active",
		Source:      "admin",
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}
	if err := db.Delete(&SubscriptionPlan{}, plan.Id).Error; err != nil {
		t.Fatalf("failed to soft delete plan: %v", err)
	}
	InvalidateSubscriptionPlanCache(plan.Id)

	info, err := GetSubscriptionPlanInfoByUserSubscriptionId(subscription.Id)
	if err != nil {
		t.Fatalf("GetSubscriptionPlanInfoByUserSubscriptionId() error = %v", err)
	}
	if info.PlanId != plan.Id {
		t.Fatalf("expected plan id %d, got %d", plan.Id, info.PlanId)
	}
	if info.PlanTitle != plan.Title {
		t.Fatalf("expected deleted plan title %q, got %q", plan.Title, info.PlanTitle)
	}
}

func TestCompleteSubscriptionOrderAllowsSoftDeletedPlan(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	user := seedReferralUser(t, db, "soft-delete-order-user", 0, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 18)
	order := seedPendingReferralOrder(t, db, user.Id, plan.Id, "trade-soft-deleted-plan", 18)

	if err := db.Delete(&SubscriptionPlan{}, plan.Id).Error; err != nil {
		t.Fatalf("failed to soft delete plan: %v", err)
	}
	InvalidateSubscriptionPlanCache(plan.Id)

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder() error = %v", err)
	}

	var count int64
	if err := db.Model(&UserSubscription{}).Where("plan_id = ? AND user_id = ?", plan.Id, user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count user subscriptions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 subscription created from soft-deleted plan, got %d", count)
	}
}
