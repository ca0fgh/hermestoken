package model

import (
	"encoding/json"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
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

func TestGetAllUserSubscriptionsIncludesDeletedPlanTitle(t *testing.T) {
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

	summaries, err := GetAllUserSubscriptions(subscription.UserId)
	if err != nil {
		t.Fatalf("GetAllUserSubscriptions() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 subscription summary, got %d", len(summaries))
	}

	payload, err := json.Marshal(summaries[0])
	if err != nil {
		t.Fatalf("failed to marshal subscription summary: %v", err)
	}
	var decoded struct {
		Plan *struct {
			Id    int    `json:"id"`
			Title string `json:"title"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode subscription summary: %v", err)
	}
	if decoded.Plan == nil {
		t.Fatalf("expected subscription summary to include plan info, got %s", payload)
	}
	if decoded.Plan.Id != plan.Id || decoded.Plan.Title != plan.Title {
		t.Fatalf("expected plan %d %q, got %+v", plan.Id, plan.Title, decoded.Plan)
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
