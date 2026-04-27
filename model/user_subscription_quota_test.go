package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestHydrateActiveSubscriptionQuotaSeparatesWalletAndSubscription(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	now := common.GetTimestamp()

	user := seedReferralUser(t, db, "quota-summary-user", 0, dto.UserSetting{})
	user.Quota = 300
	user.UsedQuota = 700
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to seed wallet quota: %v", err)
	}

	activePlan := seedReferralPlan(t, db, 1)
	expiredPlan := seedReferralPlan(t, db, 1)

	if err := db.Create(&UserSubscription{
		UserId:      user.Id,
		PlanId:      activePlan.Id,
		AmountTotal: 1000,
		AmountUsed:  250,
		StartTime:   now - 100,
		EndTime:     now + 100,
		Status:      "active",
		Source:      "order",
	}).Error; err != nil {
		t.Fatalf("failed to seed active subscription: %v", err)
	}
	if err := db.Create(&UserSubscription{
		UserId:      user.Id,
		PlanId:      expiredPlan.Id,
		AmountTotal: 9999,
		AmountUsed:  9999,
		StartTime:   now - 200,
		EndTime:     now - 100,
		Status:      "active",
		Source:      "order",
	}).Error; err != nil {
		t.Fatalf("failed to seed expired subscription: %v", err)
	}

	users := []*User{user}
	if err := HydrateActiveSubscriptionQuota(users); err != nil {
		t.Fatalf("hydrate subscription quota: %v", err)
	}

	if user.Quota != 300 || user.UsedQuota != 700 {
		t.Fatalf("wallet quota mutated: quota=%d used=%d", user.Quota, user.UsedQuota)
	}
	if user.SubscriptionAmountTotal != 1000 {
		t.Fatalf("subscription total = %d, want 1000", user.SubscriptionAmountTotal)
	}
	if user.SubscriptionAmountUsed != 250 {
		t.Fatalf("subscription used = %d, want 250", user.SubscriptionAmountUsed)
	}
	if user.SubscriptionQuotaUnlimited {
		t.Fatal("subscription unlimited = true, want false")
	}
}

func TestHydrateActiveSubscriptionQuotaMarksUnlimitedSubscription(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	now := common.GetTimestamp()

	user := seedReferralUser(t, db, "quota-unlimited-user", 0, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 1)

	if err := db.Create(&UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: 0,
		AmountUsed:  500,
		StartTime:   now - 100,
		EndTime:     now + 100,
		Status:      "active",
		Source:      "admin",
	}).Error; err != nil {
		t.Fatalf("failed to seed unlimited subscription: %v", err)
	}

	users := []*User{user}
	if err := HydrateActiveSubscriptionQuota(users); err != nil {
		t.Fatalf("hydrate subscription quota: %v", err)
	}

	if !user.SubscriptionQuotaUnlimited {
		t.Fatal("subscription unlimited = false, want true")
	}
	if user.SubscriptionAmountUsed != 500 {
		t.Fatalf("subscription used = %d, want 500", user.SubscriptionAmountUsed)
	}
	if user.SubscriptionAmountTotal != 0 {
		t.Fatalf("subscription total = %d, want 0 for unlimited", user.SubscriptionAmountTotal)
	}
}
