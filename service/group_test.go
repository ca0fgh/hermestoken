package service

import (
	"testing"

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

func TestGetUserSelectableGroupsDoesNotAutoIncludeAssignedGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	groups := GetUserSelectableGroups("default")

	if _, ok := groups["default"]; ok {
		t.Fatalf("expected assigned user group to stay hidden when it is not selectable")
	}
	if got := groups["standard"]; got != "标准价格" {
		t.Fatalf("expected standard group to remain selectable, got %q", got)
	}
}

func TestGetUserSelectableGroupsAppliesPerUserOverridesWithoutAutoFallback(t *testing.T) {
	withSelectableGroupSettings(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{"vip":{"+:exclusive":"专属分组","-:default":"默认分组"}}`,
	)

	groups := GetUserSelectableGroups("vip")

	if _, ok := groups["vip"]; ok {
		t.Fatalf("expected current user group to stay hidden when not explicitly selectable")
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected removed default group to stay hidden")
	}
	if got := groups["exclusive"]; got != "专属分组" {
		t.Fatalf("expected exclusive group to be added, got %q", got)
	}
}

func TestValidateTokenSelectableGroupRejectsImplicitNonSelectableUserGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	err := ValidateTokenSelectableGroup("default", "")

	if err == nil {
		t.Fatalf("expected blank token group to be rejected when user group is not selectable")
	}
}

func TestResolveTokenGroupForRequestRejectsBlankFallbackWhenUserGroupIsNotSelectable(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	_, err := ResolveTokenGroupForRequest("default", "")

	if err == nil {
		t.Fatalf("expected runtime fallback to be rejected when user group is not selectable")
	}
}
