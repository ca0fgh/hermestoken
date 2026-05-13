package controller

import (
	"net/http"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
)

func TestGetUserGroupsHidesAssignedDefaultGroupWhenNotExplicitlySelectable(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1}`,
	)

	user := &model.User{
		Id:       203,
		Username: "assigned_group_visible_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
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

	if _, ok := groups["default"]; ok {
		t.Fatalf("expected assigned default group to stay hidden, got %#v", groups)
	}
	if _, ok := groups["standard"]; !ok {
		t.Fatalf("expected explicitly selectable group to stay visible, got %#v", groups)
	}
}

func TestGetUserGroupsIncludesAssignedNonDefaultGroupWhenNotExplicitlySelectable(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	withControllerGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{"default":1,"standard":1,"vip":1}`,
	)

	user := &model.User{
		Id:       204,
		Username: "assigned_non_default_visible_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
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

	if _, ok := groups["vip"]; !ok {
		t.Fatalf("expected assigned non-default user group to be exposed, got %#v", groups)
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected default group to stay hidden, got %#v", groups)
	}
}
