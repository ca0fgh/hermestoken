package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestAdminCreateReferralTemplate(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "vip-direct",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 1200,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).Where("name = ?", "vip-direct").Count(&count).Error; err != nil {
		t.Fatalf("failed to count templates: %v", err)
	}
	if count != 1 {
		t.Fatalf("template count = %d, want 1", count)
	}
}

func TestAdminCreateDirectReferralTemplateAllowsOmittingTeamCap(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "vip-direct-without-team-cap",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1200,
		"invitee_share_default_bps": 500,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var template model.ReferralTemplate
	if err := db.Where("name = ?", "vip-direct-without-team-cap").First(&template).Error; err != nil {
		t.Fatalf("failed to load created template: %v", err)
	}
	if template.TeamCapBps != 0 {
		t.Fatalf("TeamCapBps = %d, want 0 for direct template", template.TeamCapBps)
	}
}

func TestAdminCreateReferralTemplateRejectsEmptyGroup(t *testing.T) {
	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "",
		"name":                      "missing-group",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatal("expected failure for empty group")
	}
}

func TestAdminCreateReferralTemplateAllowsMultipleTemplatesPerScope(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	firstBody := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "vip-direct-a",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}
	secondBody := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "vip-team-b",
		"level_type":                model.ReferralLevelTypeTeam,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", firstBody, 1)
	AdminCreateReferralTemplate(firstCtx)
	firstResponse := decodeAPIResponse(t, firstRecorder)
	if !firstResponse.Success {
		t.Fatalf("expected first create success, got message: %s", firstResponse.Message)
	}

	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", secondBody, 1)
	AdminCreateReferralTemplate(secondCtx)
	secondResponse := decodeAPIResponse(t, secondRecorder)
	if !secondResponse.Success {
		t.Fatalf("expected second create success, got message: %s", secondResponse.Message)
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).
		Where("referral_type = ? AND `group` = ?", model.ReferralTypeSubscription, "vip").
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count templates: %v", err)
	}
	if count != 2 {
		t.Fatalf("template count = %d, want 2", count)
	}
}

func TestAdminCreateReferralTemplateRejectsDuplicateName(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	firstBody := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "duplicate-template-name",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}
	secondBody := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "default",
		"name":                      "duplicate-template-name",
		"level_type":                model.ReferralLevelTypeTeam,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", firstBody, 1)
	AdminCreateReferralTemplate(firstCtx)
	firstResponse := decodeAPIResponse(t, firstRecorder)
	if !firstResponse.Success {
		t.Fatalf("expected first create success, got message: %s", firstResponse.Message)
	}

	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", secondBody, 1)
	AdminCreateReferralTemplate(secondCtx)
	secondResponse := decodeAPIResponse(t, secondRecorder)
	if secondResponse.Success {
		t.Fatal("expected duplicate name create to fail")
	}
	if secondResponse.Message != "template name already exists" {
		t.Fatalf("duplicate name message = %q, want %q", secondResponse.Message, "template name already exists")
	}
}

func TestAdminUpdateSubscriptionReferralGlobalSetting(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/settings/subscription", map[string]interface{}{
		"team_decay_ratio": 0.25,
		"team_max_depth":   0,
	}, 1)
	AdminUpdateSubscriptionReferralGlobalSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	setting := model.GetSubscriptionReferralGlobalSetting()
	if setting.TeamDecayRatio != 0.25 {
		t.Fatalf("TeamDecayRatio = %v, want 0.25", setting.TeamDecayRatio)
	}
	if setting.TeamMaxDepth != 0 {
		t.Fatalf("TeamMaxDepth = %d, want 0", setting.TeamMaxDepth)
	}
}

func TestAdminUpsertReferralTemplateBindingUsesTemplateScope(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "binding-admin-user", 0, dto.UserSetting{})
	template := &model.ReferralTemplate{
		ReferralType:           model.ReferralTypeSubscription,
		Group:                  "vip",
		Name:                   "binding-admin-template",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 600,
		CreatedBy:              1,
		UpdatedBy:              1,
	}
	if err := model.CreateReferralTemplate(template); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	body := map[string]interface{}{
		"template_id": template.Id,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/bindings/users/"+strconv.Itoa(user.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminUpsertReferralTemplateBindingForUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	view, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("failed to reload binding view: %v", err)
	}
	if view == nil {
		t.Fatal("expected binding view")
	}
	if view.Binding.TemplateId != template.Id {
		t.Fatalf("TemplateId = %d, want %d", view.Binding.TemplateId, template.Id)
	}
}
