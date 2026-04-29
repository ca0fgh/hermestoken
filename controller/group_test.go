package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
)

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

func TestGetUserTokenGroupsHidesAssignedGroupWhenNotExplicitlySelectable(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1}`,
	)

	user := &model.User{
		Id:       205,
		Username: "token-group-hidden-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/groups", nil, user.Id)
	GetUserTokenGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["default"]; ok {
		t.Fatalf("expected assigned default group to stay hidden for token selection, got %#v", groups)
	}
	if _, ok := groups["standard"]; !ok {
		t.Fatalf("expected explicitly selectable group to remain visible, got %#v", groups)
	}
}

func TestGetUserTokenGroupsIncludesAssignedNonDefaultGroup(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{"default":1,"standard":1,"cc-opus-福利渠道":1}`,
	)

	user := &model.User{
		Id:       207,
		Username: "token-group-assigned-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "cc-opus-福利渠道",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/groups", nil, user.Id)
	GetUserTokenGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["cc-opus-福利渠道"]; !ok {
		t.Fatalf("expected assigned non-default user group to stay visible for token selection, got %#v", groups)
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected default group to stay hidden for token selection, got %#v", groups)
	}
}

func TestGetUserTokenGroupsIncludesActiveSubscriptionUpgradeGroup(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1,"cc-opus4.6-福利渠道":1}`,
	)

	user := &model.User{
		Id:       206,
		Username: "token-group-subscription-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	plan := &model.SubscriptionPlan{
		Id:            306,
		Title:         "token-group-upgrade-plan",
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
		Source:        "test",
		UpgradeGroup:  "cc-opus4.6-福利渠道",
		PrevUserGroup: "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/groups", nil, user.Id)
	GetUserTokenGroups(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var groups map[string]map[string]interface{}
	if err := common.Unmarshal(response.Data, &groups); err != nil {
		t.Fatalf("failed to decode group response: %v", err)
	}

	if _, ok := groups["cc-opus4.6-福利渠道"]; !ok {
		t.Fatalf("expected active subscription upgrade group to stay visible for token selection, got %#v", groups)
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected assigned default group to stay hidden for token selection, got %#v", groups)
	}
}
