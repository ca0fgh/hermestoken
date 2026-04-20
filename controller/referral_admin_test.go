package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
	db := setupSubscriptionControllerTestDB(t)

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
	if !secondResponse.Success {
		t.Fatalf("expected same name in different groups to succeed, got %q", secondResponse.Message)
	}

	thirdBody := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "duplicate-template-name",
		"level_type":                model.ReferralLevelTypeTeam,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"invitee_share_default_bps": 0,
	}
	thirdCtx, thirdRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", thirdBody, 1)
	AdminCreateReferralTemplate(thirdCtx)
	thirdResponse := decodeAPIResponse(t, thirdRecorder)
	if thirdResponse.Success {
		t.Fatal("expected duplicate referral_type + group + name create to fail")
	}
	if thirdResponse.Message != "template name already exists" {
		t.Fatalf("duplicate name message = %q, want %q", thirdResponse.Message, "template name already exists")
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).
		Where("name = ?", "duplicate-template-name").
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count templates: %v", err)
	}
	if count != 2 {
		t.Fatalf("template count = %d, want 2", count)
	}
}

func TestAdminCreateReferralTemplateWithGroupsCreatesBundleRows(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"groups":                    []string{"default", "vip"},
		"name":                      "bundle-direct",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1100,
		"invitee_share_default_bps": 400,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var rows []model.ReferralTemplate
	if err := db.Where("name = ?", "bundle-direct").Order("`group` ASC").Find(&rows).Error; err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[0].Group != "default" || rows[1].Group != "vip" {
		t.Fatalf("groups = [%s %s], want [default vip]", rows[0].Group, rows[1].Group)
	}
	if rows[0].BundleKey == "" || rows[0].BundleKey != rows[1].BundleKey {
		t.Fatalf("expected shared non-empty bundle key, got %q and %q", rows[0].BundleKey, rows[1].BundleKey)
	}
}

func TestAdminListReferralTemplatesBundleViewAggregatesGroups(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	created, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "bundle-team",
		LevelType:              model.ReferralLevelTypeTeam,
		Enabled:                true,
		TeamCapBps:             2500,
		InviteeShareDefaultBps: 700,
	}, 1)
	if err != nil {
		t.Fatalf("failed to seed bundle: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/templates?view=bundle&referral_type="+model.ReferralTypeSubscription, nil, 1)
	AdminListReferralTemplates(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var payload struct {
		Items []model.ReferralTemplateBundle `json:"items"`
	}
	if err := common.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode response payload: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("bundle count = %d, want 1", len(payload.Items))
	}
	item := payload.Items[0]
	if item.BundleKey != created[0].BundleKey {
		t.Fatalf("bundle key = %q, want %q", item.BundleKey, created[0].BundleKey)
	}
	if strings.Join(item.Groups, ",") != "default,vip" {
		t.Fatalf("groups = %v, want [default vip]", item.Groups)
	}
	if len(item.TemplateIDs) != 2 {
		t.Fatalf("template ids = %v, want 2 ids", item.TemplateIDs)
	}
}

func TestAdminUpdateReferralTemplateBundleReplacesGroupSet(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "mutable-bundle",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           900,
		InviteeShareDefaultBps: 300,
	}, 1)
	if err != nil {
		t.Fatalf("failed to seed bundle: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"groups":                    []string{"premium", "vip"},
		"name":                      "mutable-bundle-renamed",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   false,
		"direct_cap_bps":            1400,
		"invitee_share_default_bps": 600,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/templates/"+strconv.Itoa(rows[0].Id), body, 9)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(rows[0].Id)}}
	AdminUpdateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var updatedRows []model.ReferralTemplate
	if err := db.Where("bundle_key = ?", rows[0].BundleKey).Order("`group` ASC").Find(&updatedRows).Error; err != nil {
		t.Fatalf("failed to list updated rows: %v", err)
	}
	if len(updatedRows) != 2 {
		t.Fatalf("updated row count = %d, want 2", len(updatedRows))
	}
	if strings.Join([]string{updatedRows[0].Group, updatedRows[1].Group}, ",") != "premium,vip" {
		t.Fatalf("updated groups = [%s %s], want [premium vip]", updatedRows[0].Group, updatedRows[1].Group)
	}
	for _, row := range updatedRows {
		if row.Name != "mutable-bundle-renamed" {
			t.Fatalf("row name = %q, want mutable-bundle-renamed", row.Name)
		}
		if row.Enabled {
			t.Fatal("expected updated rows to be disabled")
		}
		if row.DirectCapBps != 1400 {
			t.Fatalf("direct cap = %d, want 1400", row.DirectCapBps)
		}
		if row.InviteeShareDefaultBps != 600 {
			t.Fatalf("invitee share = %d, want 600", row.InviteeShareDefaultBps)
		}
		if row.UpdatedBy != 9 {
			t.Fatalf("updated by = %d, want 9", row.UpdatedBy)
		}
	}

	var staleCount int64
	if err := db.Model(&model.ReferralTemplate{}).
		Where("bundle_key = ? AND `group` = ?", rows[0].BundleKey, "default").
		Count(&staleCount).Error; err != nil {
		t.Fatalf("failed to count stale rows: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("stale default row count = %d, want 0", staleCount)
	}
}

func TestAdminCreateReferralTemplateWithGroupsRollsBackOnConflict(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if _, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType: model.ReferralTypeSubscription,
		Groups:       []string{"vip"},
		Name:         "conflict-name",
		LevelType:    model.ReferralLevelTypeDirect,
		Enabled:      true,
		DirectCapBps: 1000,
	}, 1); err != nil {
		t.Fatalf("failed to seed conflicting bundle: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":  model.ReferralTypeSubscription,
		"groups":         []string{"default", "vip"},
		"name":           "conflict-name",
		"level_type":     model.ReferralLevelTypeDirect,
		"enabled":        true,
		"direct_cap_bps": 1000,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatal("expected conflict create to fail")
	}
	if response.Message != "template name already exists" {
		t.Fatalf("message = %q, want %q", response.Message, "template name already exists")
	}

	var defaultCount int64
	if err := db.Model(&model.ReferralTemplate{}).
		Where("name = ? AND `group` = ?", "conflict-name", "default").
		Count(&defaultCount).Error; err != nil {
		t.Fatalf("failed to count partially inserted rows: %v", err)
	}
	if defaultCount != 0 {
		t.Fatalf("default group count = %d, want 0", defaultCount)
	}
}

func TestAdminDeleteReferralTemplateBundleByTemplateIDRemovesAllRows(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType: model.ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "delete-me",
		LevelType:    model.ReferralLevelTypeTeam,
		Enabled:      true,
		TeamCapBps:   2300,
	}, 1)
	if err != nil {
		t.Fatalf("failed to seed bundle: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/referral/templates/"+strconv.Itoa(rows[0].Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(rows[0].Id)}}
	AdminDeleteReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).
		Where("bundle_key = ?", rows[0].BundleKey).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count deleted rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("bundle row count = %d, want 0", count)
	}
}

func TestAdminDeleteReferralTemplateBundlePromotedFromLegacyRowRemovesAllRows(t *testing.T) {
	for _, tc := range []struct {
		name      string
		bundleKey string
	}{
		{name: "empty bundle key", bundleKey: ""},
		{name: "synthetic bundle key", bundleKey: "template:%d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionControllerTestDB(t)

			legacyRow := &model.ReferralTemplate{
				ReferralType:           model.ReferralTypeSubscription,
				Group:                  "legacy",
				Name:                   "legacy-promoted-bundle",
				LevelType:              model.ReferralLevelTypeDirect,
				Enabled:                true,
				DirectCapBps:           950,
				InviteeShareDefaultBps: 200,
				CreatedBy:              1,
				UpdatedBy:              1,
			}
			if err := model.CreateReferralTemplate(legacyRow); err != nil {
				t.Fatalf("failed to create legacy row: %v", err)
			}

			bundleKey := tc.bundleKey
			if strings.Contains(bundleKey, "%d") {
				bundleKey = fmt.Sprintf(bundleKey, legacyRow.Id)
			}
			if err := db.Model(&model.ReferralTemplate{}).
				Where("id = ?", legacyRow.Id).
				UpdateColumn("bundle_key", bundleKey).Error; err != nil {
				t.Fatalf("failed to rewrite bundle key: %v", err)
			}

			updateBody := map[string]interface{}{
				"referral_type":             model.ReferralTypeSubscription,
				"groups":                    []string{"default", "vip"},
				"name":                      "legacy-promoted-bundle",
				"level_type":                model.ReferralLevelTypeDirect,
				"enabled":                   false,
				"direct_cap_bps":            1500,
				"invitee_share_default_bps": 650,
			}

			updateCtx, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/templates/"+strconv.Itoa(legacyRow.Id), updateBody, 7)
			updateCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(legacyRow.Id)}}
			AdminUpdateReferralTemplate(updateCtx)

			updateResponse := decodeAPIResponse(t, updateRecorder)
			if !updateResponse.Success {
				t.Fatalf("expected update success, got message: %s", updateResponse.Message)
			}

			var promotedRows []model.ReferralTemplate
			if err := db.Where("name = ?", "legacy-promoted-bundle").Order("`group` ASC, id ASC").Find(&promotedRows).Error; err != nil {
				t.Fatalf("failed to load promoted rows: %v", err)
			}
			if len(promotedRows) != 2 {
				t.Fatalf("promoted row count = %d, want 2", len(promotedRows))
			}

			promotedBundleKey := promotedRows[0].BundleKey
			if promotedBundleKey == "" {
				t.Fatal("expected durable promoted bundle key")
			}
			if strings.HasPrefix(promotedBundleKey, "template:") {
				t.Fatalf("promoted bundle key = %q, want durable non-fallback key", promotedBundleKey)
			}
			if promotedRows[1].BundleKey != promotedBundleKey {
				t.Fatalf("bundle keys = %q and %q, want shared key", promotedRows[0].BundleKey, promotedRows[1].BundleKey)
			}

			deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/referral/templates/"+strconv.Itoa(promotedRows[0].Id), nil, 7)
			deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(promotedRows[0].Id)}}
			AdminDeleteReferralTemplate(deleteCtx)

			deleteResponse := decodeAPIResponse(t, deleteRecorder)
			if !deleteResponse.Success {
				t.Fatalf("expected delete success, got message: %s", deleteResponse.Message)
			}

			var remaining int64
			if err := db.Model(&model.ReferralTemplate{}).
				Where("bundle_key = ?", promotedBundleKey).
				Count(&remaining).Error; err != nil {
				t.Fatalf("failed to count remaining promoted rows: %v", err)
			}
			if remaining != 0 {
				t.Fatalf("remaining promoted row count = %d, want 0", remaining)
			}
		})
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
