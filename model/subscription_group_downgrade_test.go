package model

import (
	"errors"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"gorm.io/gorm"
)

func seedSubscriptionUser(t *testing.T, db *gorm.DB, username string, group string) *User {
	t.Helper()

	user := &User{
		Username: username,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    group,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func seedActiveUserSubscription(t *testing.T, db *gorm.DB, userID int, plan *SubscriptionPlan, source string) *UserSubscription {
	t.Helper()

	var subscription *UserSubscription
	if err := db.Transaction(func(tx *gorm.DB) error {
		created, err := CreateUserSubscriptionFromPlanTx(tx, userID, plan, source)
		if err != nil {
			return err
		}
		subscription = created
		return nil
	}); err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}
	return subscription
}

func TestAdminInvalidateUserSubscriptionDowngradesToRemainingActiveUpgradeGroup(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "stacked-subscription-user", "default")
	planA := seedReferralPlan(t, db, 9.9)
	setReferralPlanUpgradeGroup(t, db, planA, "cc-stack-a")
	planB := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, planB, "cc-stack-b")

	subA := seedActiveUserSubscription(t, db, user.Id, planA, "order")
	subB := seedActiveUserSubscription(t, db, user.Id, planB, "admin")

	currentGroup, err := getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to load current group: %v", err)
	}
	if currentGroup != "cc-stack-b" {
		t.Fatalf("current group = %q, want cc-stack-b", currentGroup)
	}

	if _, err := AdminInvalidateUserSubscription(subB.Id); err != nil {
		t.Fatalf("AdminInvalidateUserSubscription(%d) error = %v", subB.Id, err)
	}

	currentGroup, err = getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to reload group after invalidating newest subscription: %v", err)
	}
	if currentGroup != "cc-stack-a" {
		t.Fatalf("group after invalidating newest subscription = %q, want cc-stack-a", currentGroup)
	}

	if _, err := AdminInvalidateUserSubscription(subA.Id); err != nil {
		t.Fatalf("AdminInvalidateUserSubscription(%d) error = %v", subA.Id, err)
	}

	currentGroup, err = getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to reload group after invalidating oldest subscription: %v", err)
	}
	if currentGroup != "default" {
		t.Fatalf("group after invalidating oldest subscription = %q, want default", currentGroup)
	}
}

func TestExpireDueSubscriptionsDowngradesNestedUpgradeGroupsInOrder(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "expiring-stacked-subscription-user", "default")
	planA := seedReferralPlan(t, db, 9.9)
	setReferralPlanUpgradeGroup(t, db, planA, "cc-expire-a")
	planB := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, planB, "cc-expire-b")

	subA := seedActiveUserSubscription(t, db, user.Id, planA, "order")
	subB := seedActiveUserSubscription(t, db, user.Id, planB, "admin")

	now := common.GetTimestamp()
	if err := db.Model(&UserSubscription{}).Where("id = ?", subA.Id).Update("end_time", now-20).Error; err != nil {
		t.Fatalf("failed to update first subscription end_time: %v", err)
	}
	if err := db.Model(&UserSubscription{}).Where("id = ?", subB.Id).Update("end_time", now-10).Error; err != nil {
		t.Fatalf("failed to update second subscription end_time: %v", err)
	}

	expiredCount, err := ExpireDueSubscriptions(10)
	if err != nil {
		t.Fatalf("ExpireDueSubscriptions() error = %v", err)
	}
	if expiredCount != 2 {
		t.Fatalf("expiredCount = %d, want 2", expiredCount)
	}

	currentGroup, err := getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to reload group after expiry: %v", err)
	}
	if currentGroup != "default" {
		t.Fatalf("group after expiring stacked subscriptions = %q, want default", currentGroup)
	}

	var expiredSubscriptions []UserSubscription
	if err := db.Where("user_id = ?", user.Id).Order("id asc").Find(&expiredSubscriptions).Error; err != nil {
		t.Fatalf("failed to reload expired subscriptions: %v", err)
	}
	if len(expiredSubscriptions) != 2 {
		t.Fatalf("expired subscription count = %d, want 2", len(expiredSubscriptions))
	}
	for _, subscription := range expiredSubscriptions {
		if subscription.Status != "expired" {
			t.Fatalf("subscription %d status = %q, want expired", subscription.Id, subscription.Status)
		}
	}
}

func TestPreConsumeUserSubscriptionExhaustsInsufficientSubscriptionAndDowngradesGroup(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "quota-exhausted-subscription-user", "default")
	plan := seedReferralPlan(t, db, 9.9)
	plan.TotalAmount = 100
	setReferralPlanUpgradeGroup(t, db, plan, "cc-quota-exhausted")

	sub := seedActiveUserSubscription(t, db, user.Id, plan, "order")
	if err := db.Model(&UserSubscription{}).
		Where("id = ?", sub.Id).
		Update("amount_used", int64(80)).Error; err != nil {
		t.Fatalf("failed to seed subscription usage: %v", err)
	}

	_, err := PreConsumeUserSubscription("quota-exhausted-request", user.Id, "claude-sonnet-4-6", 0, 30)
	if !errors.Is(err, ErrSubscriptionQuotaInsufficient) {
		t.Fatalf("PreConsumeUserSubscription() error = %v, want ErrSubscriptionQuotaInsufficient", err)
	}

	var reloadedSub UserSubscription
	if err := db.Where("id = ?", sub.Id).First(&reloadedSub).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if reloadedSub.Status != "expired" {
		t.Fatalf("subscription status = %q, want expired", reloadedSub.Status)
	}
	if reloadedSub.AmountUsed != reloadedSub.AmountTotal {
		t.Fatalf("subscription amount_used = %d, want amount_total %d", reloadedSub.AmountUsed, reloadedSub.AmountTotal)
	}

	currentGroup, err := getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to reload user group: %v", err)
	}
	if currentGroup != "default" {
		t.Fatalf("group after quota exhaustion = %q, want default", currentGroup)
	}

	hasActive, err := HasActiveUserSubscription(user.Id)
	if err != nil {
		t.Fatalf("HasActiveUserSubscription() error = %v", err)
	}
	if hasActive {
		t.Fatal("expected exhausted subscription to stop counting as active")
	}
}

func TestPreConsumeUserSubscriptionExhaustsStackedSubscriptionsBackToOriginalGroup(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "stacked-quota-exhausted-user", "default")
	planA := seedReferralPlan(t, db, 9.9)
	planA.TotalAmount = 100
	setReferralPlanUpgradeGroup(t, db, planA, "cc-stack-exhausted-a")
	planB := seedReferralPlan(t, db, 19.9)
	planB.TotalAmount = 100
	setReferralPlanUpgradeGroup(t, db, planB, "cc-stack-exhausted-b")

	subA := seedActiveUserSubscription(t, db, user.Id, planA, "order")
	subB := seedActiveUserSubscription(t, db, user.Id, planB, "admin")
	for _, sub := range []*UserSubscription{subA, subB} {
		if err := db.Model(&UserSubscription{}).
			Where("id = ?", sub.Id).
			Update("amount_used", int64(90)).Error; err != nil {
			t.Fatalf("failed to seed subscription %d usage: %v", sub.Id, err)
		}
	}

	_, err := PreConsumeUserSubscription("stacked-quota-exhausted-request", user.Id, "claude-sonnet-4-6", 0, 20)
	if !errors.Is(err, ErrSubscriptionQuotaInsufficient) {
		t.Fatalf("PreConsumeUserSubscription() error = %v, want ErrSubscriptionQuotaInsufficient", err)
	}

	currentGroup, err := getUserGroupByIdTx(db, user.Id)
	if err != nil {
		t.Fatalf("failed to reload user group: %v", err)
	}
	if currentGroup != "default" {
		t.Fatalf("group after exhausting stacked subscriptions = %q, want default", currentGroup)
	}

	var subs []UserSubscription
	if err := db.Where("user_id = ?", user.Id).Order("id asc").Find(&subs).Error; err != nil {
		t.Fatalf("failed to reload subscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("subscription count = %d, want 2", len(subs))
	}
	for _, sub := range subs {
		if sub.Status != "expired" {
			t.Fatalf("subscription %d status = %q, want expired", sub.Id, sub.Status)
		}
		if sub.AmountUsed != sub.AmountTotal {
			t.Fatalf("subscription %d amount_used = %d, want amount_total %d", sub.Id, sub.AmountUsed, sub.AmountTotal)
		}
	}
}
