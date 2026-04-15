package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type groupCatalogItem struct {
	GroupKey    string `json:"group_key"`
	DisplayName string `json:"display_name"`
	Status      int    `json:"status"`
}

func withControllerGroupSettingsAndRatios(t *testing.T, usableJSON string, ratioJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func TestGetUserGroupsIncludesPlanUpgradeGroupWhenSubscriptionSnapshotIsBlank(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1,"cc-opus4.6-福利渠道":1}`,
	)

	user := &model.User{
		Id:       201,
		Username: "legacy_upgrade_group_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	plan := &model.SubscriptionPlan{
		Id:            301,
		Title:         "legacy-upgrade-plan",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  "cc-opus4.6-福利渠道",
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	subscription := &model.UserSubscription{
		UserId:        user.Id,
		PlanId:        plan.Id,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     now,
		EndTime:       now + 3600,
		Status:        "active",
		Source:        "legacy-import",
		UpgradeGroup:  "",
		PrevUserGroup: "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self/groups", nil, user.Id)
	GetUserGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["cc-opus4.6-福利渠道"]; !ok {
		t.Fatalf("expected legacy subscription plan upgrade group to be exposed, got %#v", groups)
	}
}

func TestGetUserGroupsFallsBackToPlanUpgradeGroupWhenSubscriptionSnapshotIsInvalid(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1,"cc-opus4.6-福利渠道":1}`,
	)

	user := &model.User{
		Id:       202,
		Username: "stale_upgrade_group_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	plan := &model.SubscriptionPlan{
		Id:            302,
		Title:         "stale-upgrade-plan",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  "cc-opus4.6-福利渠道",
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	subscription := &model.UserSubscription{
		UserId:        user.Id,
		PlanId:        plan.Id,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     now,
		EndTime:       now + 3600,
		Status:        "active",
		Source:        "legacy-import",
		UpgradeGroup:  "cc-oups4.6-福利渠道",
		PrevUserGroup: "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self/groups", nil, user.Id)
	GetUserGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["cc-opus4.6-福利渠道"]; !ok {
		t.Fatalf("expected current plan upgrade group to be exposed, got %#v", groups)
	}
	if _, ok := groups["cc-oups4.6-福利渠道"]; ok {
		t.Fatalf("expected stale snapshot group to stay hidden, got %#v", groups)
	}
}

func TestGetUserGroupsUsesCanonicalSubscriptionSnapshotBeforeLegacyString(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1,"premium":1}`,
	)

	user := &model.User{
		Id:       203,
		Username: "canonical_snapshot_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	plan := &model.SubscriptionPlan{
		Id:              303,
		Title:           "canonical-snapshot-plan",
		PriceAmount:     9.9,
		Currency:        "USD",
		DurationUnit:    model.SubscriptionDurationMonth,
		DurationValue:   1,
		Enabled:         true,
		UpgradeGroup:    "default",
		UpgradeGroupKey: "default",
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	subscription := &model.UserSubscription{
		UserId:                   user.Id,
		PlanId:                   plan.Id,
		AmountTotal:              100,
		AmountUsed:               0,
		StartTime:                now,
		EndTime:                  now + 3600,
		Status:                   "active",
		Source:                   "migration",
		UpgradeGroup:             "legacy-premium",
		UpgradeGroupKeySnapshot:  "premium",
		UpgradeGroupNameSnapshot: "Premium",
		PrevUserGroup:            "default",
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self/groups", nil, user.Id)
	GetUserGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["premium"]; !ok {
		t.Fatalf("expected canonical snapshot key to be exposed, got %#v", groups)
	}
	if _, ok := groups["legacy-premium"]; ok {
		t.Fatalf("expected stale legacy snapshot string to stay hidden, got %#v", groups)
	}
}

func TestGetGroupsReturnsCanonicalGroupMetadata(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	group := seedSubscriptionPricingGroup(t, db, "premium")
	group.DisplayName = "Premium"
	group.Status = model.PricingGroupStatusActive
	if err := db.Save(&group).Error; err != nil {
		t.Fatalf("failed to update seeded pricing group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/group/", nil, 1)
	GetGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups []groupCatalogItem
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group catalog response: %v", err)
	}
	if len(groups) == 0 {
		t.Fatalf("expected canonical group catalog entries")
	}
	if groups[0].GroupKey != "premium" {
		t.Fatalf("expected first group_key premium, got %q", groups[0].GroupKey)
	}
	if groups[0].DisplayName != "Premium" {
		t.Fatalf("expected display_name Premium, got %q", groups[0].DisplayName)
	}
}
