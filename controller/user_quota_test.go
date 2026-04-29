package controller

import (
	"net/http"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
)

func seedRootControllerUser(t *testing.T, username string) *model.User {
	t.Helper()

	root := seedSubscriptionReferralControllerUser(t, username, 0, dto.UserSetting{})
	root.Role = common.RoleRootUser
	if err := model.DB.Save(root).Error; err != nil {
		t.Fatalf("failed to promote root user: %v", err)
	}
	return root
}

func TestUpdateUserPreservesQuotaAfterManageAdjustment(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	root := seedRootControllerUser(t, "root-manage-quota")
	target := seedSubscriptionReferralControllerUser(t, "quota-target", 0, dto.UserSetting{})
	target.Quota = 100
	if err := model.DB.Save(target).Error; err != nil {
		t.Fatalf("failed to seed target quota: %v", err)
	}

	manageCtx, manageRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     target.Id,
		"action": "add_quota",
		"mode":   "add",
		"value":  50,
	}, root.Id)
	manageCtx.Set("role", common.RoleRootUser)
	manageCtx.Set("username", root.Username)
	ManageUser(manageCtx)

	manageResp := decodeAPIResponse(t, manageRecorder)
	if !manageResp.Success {
		t.Fatalf("expected quota manage success, got message: %s", manageResp.Message)
	}

	var afterManage model.User
	if err := model.DB.First(&afterManage, target.Id).Error; err != nil {
		t.Fatalf("failed to reload user after manage quota: %v", err)
	}
	if afterManage.Quota != 150 {
		t.Fatalf("quota after manage = %d, want 150", afterManage.Quota)
	}

	updateCtx, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           target.Id,
		"username":     target.Username,
		"password":     "",
		"display_name": "updated-display-name",
		"group":        target.Group,
		"inviter_id":   target.InviterId,
		"remark":       "updated-remark",
	}, root.Id)
	updateCtx.Set("role", common.RoleRootUser)
	UpdateUser(updateCtx)

	updateResp := decodeAPIResponse(t, updateRecorder)
	if !updateResp.Success {
		t.Fatalf("expected update user success, got message: %s", updateResp.Message)
	}

	var refreshed model.User
	if err := model.DB.First(&refreshed, target.Id).Error; err != nil {
		t.Fatalf("failed to reload user after generic update: %v", err)
	}
	if refreshed.Quota != 150 {
		t.Fatalf("quota after generic update = %d, want preserved 150", refreshed.Quota)
	}
}

func TestUpdateUserIgnoresQuotaPayload(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	root := seedRootControllerUser(t, "root-ignore-quota")
	target := seedSubscriptionReferralControllerUser(t, "quota-ignore-target", 0, dto.UserSetting{})
	target.Quota = 300
	if err := model.DB.Save(target).Error; err != nil {
		t.Fatalf("failed to seed target quota: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           target.Id,
		"username":     target.Username,
		"password":     "",
		"display_name": target.DisplayName,
		"group":        target.Group,
		"quota":        0,
		"inviter_id":   target.InviterId,
		"remark":       "ignore-client-quota",
	}, root.Id)
	ctx.Set("role", common.RoleRootUser)
	UpdateUser(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected update user success, got message: %s", resp.Message)
	}

	var refreshed model.User
	if err := model.DB.First(&refreshed, target.Id).Error; err != nil {
		t.Fatalf("failed to reload user after update: %v", err)
	}
	if refreshed.Quota != 300 {
		t.Fatalf("quota after explicit quota payload = %d, want preserved 300", refreshed.Quota)
	}
}
