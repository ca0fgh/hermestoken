package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestMergePricingGroupMovesLegacyReferencesAndCreatesAlias(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}

	target := seedSubscriptionPricingGroup(t, db, "cc-opus4.6")
	source := seedSubscriptionPricingGroup(t, db, "cc-legacy")

	user := &model.User{
		Id:       901,
		Username: "merge-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    source.GroupKey,
		GroupKey: source.GroupKey,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token := &model.Token{
		UserId:             user.Id,
		Name:               "merge-token",
		Key:                "merge-token-key",
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        100,
		UnlimitedQuota:     true,
		SelectionMode:      "fixed",
		Group:              source.GroupKey,
		GroupKey:           source.GroupKey,
		ModelLimitsEnabled: false,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	plan := &model.SubscriptionPlan{
		Title:           "merge-plan",
		PriceAmount:     9.9,
		Currency:        "USD",
		DurationUnit:    model.SubscriptionDurationMonth,
		DurationValue:   1,
		Enabled:         true,
		UpgradeGroup:    source.GroupKey,
		UpgradeGroupKey: source.GroupKey,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	subscription := &model.UserSubscription{
		UserId:                   user.Id,
		PlanId:                   plan.Id,
		AmountTotal:              100,
		AmountUsed:               0,
		StartTime:                1,
		EndTime:                  9999999999,
		Status:                   "active",
		Source:                   "test",
		UpgradeGroup:             source.GroupKey,
		UpgradeGroupKeySnapshot:  source.GroupKey,
		UpgradeGroupNameSnapshot: "Legacy",
		PrevUserGroup:            "default",
		CreatedAt:                1,
		UpdatedAt:                1,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		"/api/group/admin/merge",
		map[string]any{
			"source_group_key": source.GroupKey,
			"target_group_key": target.GroupKey,
		},
		1,
	)
	MergePricingGroup(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got %s", resp.Message)
	}

	var alias model.PricingGroupAlias
	if err := db.Where("alias_key = ?", source.GroupKey).First(&alias).Error; err != nil {
		t.Fatalf("expected source alias to be created: %v", err)
	}
	if alias.GroupId != target.Id {
		t.Fatalf("expected alias group_id=%d, got %d", target.Id, alias.GroupId)
	}

	var sourceCount int64
	if err := db.Model(&model.PricingGroup{}).Where("group_key = ?", source.GroupKey).Count(&sourceCount).Error; err != nil {
		t.Fatalf("failed to count source pricing groups: %v", err)
	}
	if sourceCount != 0 {
		t.Fatalf("expected source pricing group to be removed after merge, got %d rows", sourceCount)
	}

	var reloadedUser model.User
	if err := db.Where("id = ?", user.Id).First(&reloadedUser).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloadedUser.Group != target.GroupKey || reloadedUser.GroupKey != target.GroupKey {
		t.Fatalf("expected user group fields to move to target, got group=%q group_key=%q", reloadedUser.Group, reloadedUser.GroupKey)
	}

	var reloadedToken model.Token
	if err := db.Where("id = ?", token.Id).First(&reloadedToken).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if reloadedToken.Group != target.GroupKey || reloadedToken.GroupKey != target.GroupKey {
		t.Fatalf("expected token group fields to move to target, got group=%q group_key=%q", reloadedToken.Group, reloadedToken.GroupKey)
	}

	var reloadedPlan model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&reloadedPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if reloadedPlan.UpgradeGroup != target.GroupKey || reloadedPlan.UpgradeGroupKey != target.GroupKey {
		t.Fatalf("expected plan upgrade group fields to move to target, got upgrade_group=%q upgrade_group_key=%q", reloadedPlan.UpgradeGroup, reloadedPlan.UpgradeGroupKey)
	}

	var reloadedSubscription model.UserSubscription
	if err := db.Where("id = ?", subscription.Id).First(&reloadedSubscription).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if reloadedSubscription.UpgradeGroup != target.GroupKey || reloadedSubscription.UpgradeGroupKeySnapshot != target.GroupKey {
		t.Fatalf("expected subscription upgrade fields to move to target, got upgrade_group=%q upgrade_group_key_snapshot=%q", reloadedSubscription.UpgradeGroup, reloadedSubscription.UpgradeGroupKeySnapshot)
	}
}

func TestAdminUpdatePricingGroupKeepsGroupKeyImmutable(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	group := seedSubscriptionPricingGroup(t, db, "cc-opus4.6")
	group.DisplayName = "旧名"
	group.BillingRatio = 1
	group.UserSelectable = false
	if err := db.Save(&group).Error; err != nil {
		t.Fatalf("failed to update seed group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/group/admin/"+group.GroupKey,
		map[string]any{
			"group_key":       "cc-hacked",
			"display_name":    "新名",
			"billing_ratio":   0.8,
			"user_selectable": true,
		},
		1,
	)
	ctx.Params = gin.Params{{Key: "group_key", Value: group.GroupKey}}
	AdminUpdatePricingGroup(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got %s", resp.Message)
	}

	var updated model.PricingGroup
	if err := db.Where("group_key = ?", group.GroupKey).First(&updated).Error; err != nil {
		t.Fatalf("expected original group key to remain, got error: %v", err)
	}
	if updated.DisplayName != "新名" {
		t.Fatalf("expected display_name to update, got %q", updated.DisplayName)
	}
	if updated.BillingRatio != 0.8 {
		t.Fatalf("expected billing_ratio=0.8, got %v", updated.BillingRatio)
	}
	if !updated.UserSelectable {
		t.Fatalf("expected user_selectable=true")
	}

	var hackedCount int64
	if err := db.Model(&model.PricingGroup{}).Where("group_key = ?", "cc-hacked").Count(&hackedCount).Error; err != nil {
		t.Fatalf("failed to count hacked group rows: %v", err)
	}
	if hackedCount != 0 {
		t.Fatalf("expected immutable group_key to prevent creating cc-hacked, got %d rows", hackedCount)
	}
}
