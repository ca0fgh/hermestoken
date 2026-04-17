package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

func withSelectableGroupSettings(t *testing.T, usableJSON string, specialJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalSpecial := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, specialJSON); err != nil {
		t.Fatalf("failed to set special usable groups: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecial); err != nil {
			t.Fatalf("failed to restore special usable groups: %v", err)
		}
	})
}

func withSelectableGroupSettingsAndRatios(t *testing.T, usableJSON string, specialJSON string, ratioJSON string) {
	t.Helper()

	originalRatios := ratio_setting.GroupRatio2JSONString()
	withSelectableGroupSettings(t, usableJSON, specialJSON)

	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func seedGroupTestUser(t *testing.T, id int, group string) {
	t.Helper()

	user := &model.User{
		Id:          id,
		Username:    "group_test_user",
		Password:    "password123",
		DisplayName: "group_test_user",
		Group:       group,
		Status:      1,
		Role:        1,
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
}

func seedGroupTestSubscription(t *testing.T, userID int, upgradeGroup string, status string, endTime int64) {
	t.Helper()

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:        userID,
		PlanId:        1,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     now,
		EndTime:       endTime,
		Status:        status,
		Source:        "test",
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to seed subscription: %v", err)
	}
}

func TestGetUserSelectableGroupsIncludesAssignedGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	groups := GetUserSelectableGroups("default")

	if got := groups["default"]; got != "用户分组" {
		t.Fatalf("expected assigned user group to stay visible, got %q", got)
	}
	if got := groups["standard"]; got != "标准价格" {
		t.Fatalf("expected standard group to remain selectable, got %q", got)
	}
}

func TestGetUserSelectableGroupsAppliesPerUserOverridesAndKeepsAssignedGroupVisible(t *testing.T) {
	withSelectableGroupSettings(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{"vip":{"+:exclusive":"专属分组","-:default":"默认分组"}}`,
	)

	groups := GetUserSelectableGroups("vip")

	if got := groups["vip"]; got != "用户分组" {
		t.Fatalf("expected current user group to stay visible, got %q", got)
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected removed default group to stay hidden")
	}
	if got := groups["exclusive"]; got != "专属分组" {
		t.Fatalf("expected exclusive group to be added, got %q", got)
	}
}

func TestValidateTokenSelectableGroupAllowsImplicitAssignedUserGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	err := ValidateTokenSelectableGroup("default", "")

	if err != nil {
		t.Fatalf("expected blank token group to fall back to assigned user group, got %v", err)
	}
}

func TestResolveTokenGroupForRequestUsesAssignedUserGroupAsBlankFallback(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	resolvedGroup, err := ResolveTokenGroupForRequest("default", "")

	if err != nil {
		t.Fatalf("expected runtime fallback to use assigned user group, got %v", err)
	}
	if resolvedGroup != "default" {
		t.Fatalf("expected resolved group default, got %q", resolvedGroup)
	}
}

func TestGetUserSelectableGroupsForUserIncludesActiveSubscriptionUpgradeGroup(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 101, "default")
	seedGroupTestSubscription(t, 101, "codex-5.4", "active", time.Now().Add(time.Hour).Unix())

	groups := GetUserSelectableGroupsForUser(101, "default")

	if got := groups["codex-5.4"]; got != "codex-5.4" {
		t.Fatalf("expected active subscription upgrade group to be selectable, got %q", got)
	}
}

func TestValidateTokenSelectableGroupForUserAllowsSubscriptionBackedFallback(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 102, "codex-5.4")
	seedGroupTestSubscription(t, 102, "codex-5.4", "active", time.Now().Add(time.Hour).Unix())

	if err := ValidateTokenSelectableGroupForUser(102, "codex-5.4", ""); err != nil {
		t.Fatalf("expected implicit fallback to active subscription group to pass, got %v", err)
	}

	resolvedGroup, err := ResolveTokenGroupForUserRequest(102, "codex-5.4", "")
	if err != nil {
		t.Fatalf("expected runtime fallback to active subscription group to pass, got %v", err)
	}
	if resolvedGroup != "codex-5.4" {
		t.Fatalf("expected resolved group codex-5.4, got %q", resolvedGroup)
	}
}

func TestGetUserSelectableGroupsForUserSkipsExpiredSubscriptionUpgradeGroup(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 103, "default")
	seedGroupTestSubscription(t, 103, "codex-5.4", "active", time.Now().Add(-time.Hour).Unix())

	groups := GetUserSelectableGroupsForUser(103, "default")

	if _, ok := groups["codex-5.4"]; ok {
		t.Fatalf("expected expired subscription upgrade group to stay hidden")
	}
}

func TestGetUserSelectableGroupsForUserFallsBackToPlanUpgradeGroupWhenSnapshotIsInvalid(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"cc-opus4.6-福利渠道":1}`,
	)
	seedGroupTestUser(t, 104, "default")
	plan := &model.SubscriptionPlan{
		Id:            204,
		Title:         "stale-upgrade-plan",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  "cc-opus4.6-福利渠道",
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	sub := &model.UserSubscription{
		UserId:        104,
		PlanId:        plan.Id,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     time.Now().Unix(),
		EndTime:       time.Now().Add(time.Hour).Unix(),
		Status:        "active",
		Source:        "test",
		UpgradeGroup:  "cc-oups4.6-福利渠道",
		PrevUserGroup: "default",
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to seed stale subscription snapshot: %v", err)
	}

	groups := GetUserSelectableGroupsForUser(104, "default")

	if _, ok := groups["cc-opus4.6-福利渠道"]; !ok {
		t.Fatalf("expected current plan upgrade group to be selectable, got %#v", groups)
	}
	if _, ok := groups["cc-oups4.6-福利渠道"]; ok {
		t.Fatalf("expected stale snapshot group to stay hidden, got %#v", groups)
	}
}
