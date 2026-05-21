package model

import (
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"gorm.io/gorm"
)

func seedSubscriptionToken(t *testing.T, db *gorm.DB, token *Token) *Token {
	t.Helper()

	if token.Key == "" {
		token.Key = token.Name + "_key"
	}
	if token.CreatedTime == 0 {
		token.CreatedTime = common.GetTimestamp()
	}
	if token.AccessedTime == 0 {
		token.AccessedTime = token.CreatedTime
	}
	token.UnlimitedQuota = true
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token %q: %v", token.Name, err)
	}
	return token
}

func TestCreateUserSubscriptionExtendsFiniteTokensToSubscriptionEnd(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "subscription-token-expiry-user", "default")
	plan := seedReferralPlan(t, db, 9.9)
	setReferralPlanUpgradeGroup(t, db, plan, "cc-token-expiry")

	now := common.GetTimestamp()
	expiredToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "expired-subscription-bound-token",
		Status:      common.TokenStatusExpired,
		ExpiredTime: now - 60,
		Group:       "cc-token-expiry",
	})
	shortToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "short-subscription-bound-token",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: now + int64(time.Hour/time.Second),
		Group:       "cc-token-expiry",
	})
	neverExpireToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "never-expire-token",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		Group:       "cc-token-expiry",
	})
	longTokenExpiry := now + int64((90*24*time.Hour)/time.Second)
	longToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "long-token",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: longTokenExpiry,
		Group:       "cc-token-expiry",
	})
	disabledTokenExpiry := now - 30
	disabledToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "disabled-token",
		Status:      common.TokenStatusDisabled,
		ExpiredTime: disabledTokenExpiry,
		Group:       "cc-token-expiry",
	})
	unrelatedGroupTokenExpiry := now - 10
	unrelatedGroupToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "unrelated-group-token",
		Status:      common.TokenStatusExpired,
		ExpiredTime: unrelatedGroupTokenExpiry,
		Group:       "other-paid-group",
	})

	subscription := seedActiveUserSubscription(t, db, user.Id, plan, "order")

	var tokens []Token
	if err := db.Where("user_id = ?", user.Id).Find(&tokens).Error; err != nil {
		t.Fatalf("failed to reload tokens: %v", err)
	}
	tokensByID := make(map[int]Token, len(tokens))
	for _, token := range tokens {
		tokensByID[token.Id] = token
	}

	reloadedExpiredToken := tokensByID[expiredToken.Id]
	if reloadedExpiredToken.ExpiredTime != subscription.EndTime {
		t.Fatalf("expired token expired_time = %d, want subscription end %d", reloadedExpiredToken.ExpiredTime, subscription.EndTime)
	}
	if reloadedExpiredToken.Status != common.TokenStatusEnabled {
		t.Fatalf("expired token status = %d, want enabled", reloadedExpiredToken.Status)
	}

	reloadedShortToken := tokensByID[shortToken.Id]
	if reloadedShortToken.ExpiredTime != subscription.EndTime {
		t.Fatalf("short token expired_time = %d, want subscription end %d", reloadedShortToken.ExpiredTime, subscription.EndTime)
	}

	reloadedNeverExpireToken := tokensByID[neverExpireToken.Id]
	if reloadedNeverExpireToken.ExpiredTime != -1 {
		t.Fatalf("never-expire token expired_time = %d, want -1", reloadedNeverExpireToken.ExpiredTime)
	}

	reloadedLongToken := tokensByID[longToken.Id]
	if reloadedLongToken.ExpiredTime != longTokenExpiry {
		t.Fatalf("long token expired_time = %d, want unchanged %d", reloadedLongToken.ExpiredTime, longTokenExpiry)
	}

	reloadedDisabledToken := tokensByID[disabledToken.Id]
	if reloadedDisabledToken.Status != common.TokenStatusDisabled {
		t.Fatalf("disabled token status = %d, want disabled", reloadedDisabledToken.Status)
	}
	if reloadedDisabledToken.ExpiredTime != disabledTokenExpiry {
		t.Fatalf("disabled token expired_time = %d, want unchanged %d", reloadedDisabledToken.ExpiredTime, disabledTokenExpiry)
	}

	reloadedUnrelatedGroupToken := tokensByID[unrelatedGroupToken.Id]
	if reloadedUnrelatedGroupToken.Status != common.TokenStatusExpired {
		t.Fatalf("unrelated group token status = %d, want expired", reloadedUnrelatedGroupToken.Status)
	}
	if reloadedUnrelatedGroupToken.ExpiredTime != unrelatedGroupTokenExpiry {
		t.Fatalf("unrelated group token expired_time = %d, want unchanged %d", reloadedUnrelatedGroupToken.ExpiredTime, unrelatedGroupTokenExpiry)
	}
}

func TestCreateUserSubscriptionExtendsTokensToLatestActiveSubscriptionEnd(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "subscription-token-latest-end-user", "default")
	plan := seedReferralPlan(t, db, 9.9)

	now := common.GetTimestamp()
	latestActiveEnd := now + int64((60*24*time.Hour)/time.Second)
	if err := db.Create(&UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: 100,
		AmountUsed:  0,
		StartTime:   now,
		EndTime:     latestActiveEnd,
		Status:      "active",
		Source:      "test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("failed to seed later active subscription: %v", err)
	}
	token := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "latest-active-end-token",
		Status:      common.TokenStatusExpired,
		ExpiredTime: now - 60,
	})

	created := seedActiveUserSubscription(t, db, user.Id, plan, "order")
	if created.EndTime >= latestActiveEnd {
		t.Fatalf("test setup invalid: created subscription end %d should be before latest active end %d", created.EndTime, latestActiveEnd)
	}

	var reloaded Token
	if err := db.Where("id = ?", token.Id).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if reloaded.ExpiredTime != latestActiveEnd {
		t.Fatalf("token expired_time = %d, want latest active subscription end %d", reloaded.ExpiredTime, latestActiveEnd)
	}
	if reloaded.Status != common.TokenStatusEnabled {
		t.Fatalf("token status = %d, want enabled", reloaded.Status)
	}
}

func TestCreateUserSubscriptionExtendsConcreteTokenToMatchingSubscriptionGroupEnd(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	user := seedSubscriptionUser(t, db, "subscription-token-matching-group-user", "default")
	planA := seedReferralPlan(t, db, 9.9)
	setReferralPlanUpgradeGroup(t, db, planA, "cc-group-a")
	planB := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, planB, "cc-group-b")

	now := common.GetTimestamp()
	groupAEnd := now + int64((45*24*time.Hour)/time.Second)
	groupBEnd := now + int64((70*24*time.Hour)/time.Second)
	for _, sub := range []UserSubscription{
		{
			UserId:       user.Id,
			PlanId:       planA.Id,
			AmountTotal:  100,
			StartTime:    now,
			EndTime:      groupAEnd,
			Status:       "active",
			Source:       "test",
			UpgradeGroup: "cc-group-a",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserId:       user.Id,
			PlanId:       planB.Id,
			AmountTotal:  100,
			StartTime:    now,
			EndTime:      groupBEnd,
			Status:       "active",
			Source:       "test",
			UpgradeGroup: "cc-group-b",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	} {
		if err := db.Create(&sub).Error; err != nil {
			t.Fatalf("failed to seed active subscription: %v", err)
		}
	}
	groupAToken := seedSubscriptionToken(t, db, &Token{
		UserId:      user.Id,
		Name:        "group-a-token",
		Status:      common.TokenStatusExpired,
		ExpiredTime: now - 60,
		Group:       "cc-group-a",
	})

	created := seedActiveUserSubscription(t, db, user.Id, planA, "order")
	if created.EndTime >= groupAEnd {
		t.Fatalf("test setup invalid: created subscription end %d should be before group A end %d", created.EndTime, groupAEnd)
	}

	var reloaded Token
	if err := db.Where("id = ?", groupAToken.Id).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if reloaded.ExpiredTime != groupAEnd {
		t.Fatalf("group A token expired_time = %d, want matching group end %d", reloaded.ExpiredTime, groupAEnd)
	}
	if reloaded.ExpiredTime == groupBEnd {
		t.Fatalf("group A token was extended to unrelated group B end %d", groupBEnd)
	}
}
