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

func TestAdminUpdateReferralTemplateBundleRemovesStaleBindings(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "binding-cleanup-update",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           900,
		InviteeShareDefaultBps: 300,
	}, 1)
	if err != nil {
		t.Fatalf("failed to seed bundle: %v", err)
	}

	user := seedSubscriptionReferralControllerUser(t, "binding-cleanup-update-user", 0, dto.UserSetting{})
	var defaultRow model.ReferralTemplate
	for _, row := range rows {
		if row.Group == "default" {
			defaultRow = row
			break
		}
	}
	if defaultRow.Id == 0 {
		t.Fatal("expected default row in seeded bundle")
	}

	if _, err := model.UpsertReferralTemplateBinding(&model.ReferralTemplateBinding{
		UserId:     user.Id,
		TemplateId: defaultRow.Id,
		CreatedBy:  1,
		UpdatedBy:  1,
	}); err != nil {
		t.Fatalf("failed to create binding: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"groups":                    []string{"premium", "vip"},
		"name":                      "binding-cleanup-update",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            900,
		"invitee_share_default_bps": 300,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/templates/"+strconv.Itoa(defaultRow.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(defaultRow.Id)}}
	AdminUpdateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	active, binding, err := model.HasActiveReferralTemplateBinding(user.Id, model.ReferralTypeSubscription, "default")
	if err != nil {
		t.Fatalf("HasActiveReferralTemplateBinding() error = %v", err)
	}
	if active {
		t.Fatal("expected removed-group binding to be inactive")
	}
	if binding != nil {
		t.Fatalf("expected stale binding cleanup, got binding %+v", *binding)
	}

	views, err := model.ListReferralTemplateBindingsByUser(user.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingsByUser() error = %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("binding view count = %d, want 0 after stale binding cleanup", len(views))
	}

	view, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, "default")
	if err != nil {
		t.Fatalf("GetReferralTemplateBindingViewByUserAndScope() error = %v", err)
	}
	if view != nil {
		t.Fatal("expected nil binding view after stale binding cleanup")
	}

	var bindingCount int64
	if err := db.Model(&model.ReferralTemplateBinding{}).
		Where("user_id = ? AND referral_type = ? AND `group` = ?", user.Id, model.ReferralTypeSubscription, "default").
		Count(&bindingCount).Error; err != nil {
		t.Fatalf("failed to count bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("binding count = %d, want 0 after stale binding cleanup", bindingCount)
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

func TestAdminDeleteReferralTemplateBundleRemovesStaleBindings(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "binding-cleanup-delete",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1100,
		InviteeShareDefaultBps: 500,
	}, 1)
	if err != nil {
		t.Fatalf("failed to seed bundle: %v", err)
	}

	user := seedSubscriptionReferralControllerUser(t, "binding-cleanup-delete-user", 0, dto.UserSetting{})
	if _, err := model.UpsertReferralTemplateBinding(&model.ReferralTemplateBinding{
		UserId:     user.Id,
		TemplateId: rows[0].Id,
		CreatedBy:  1,
		UpdatedBy:  1,
	}); err != nil {
		t.Fatalf("failed to create binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/referral/templates/"+strconv.Itoa(rows[0].Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(rows[0].Id)}}
	AdminDeleteReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	active, binding, err := model.HasActiveReferralTemplateBinding(user.Id, model.ReferralTypeSubscription, rows[0].Group)
	if err != nil {
		t.Fatalf("HasActiveReferralTemplateBinding() error = %v", err)
	}
	if active {
		t.Fatal("expected deleted-bundle binding to be inactive")
	}
	if binding != nil {
		t.Fatalf("expected binding cleanup after bundle delete, got binding %+v", *binding)
	}

	views, err := model.ListReferralTemplateBindingsByUser(user.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingsByUser() error = %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("binding view count = %d, want 0 after bundle delete cleanup", len(views))
	}

	view, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, rows[0].Group)
	if err != nil {
		t.Fatalf("GetReferralTemplateBindingViewByUserAndScope() error = %v", err)
	}
	if view != nil {
		t.Fatal("expected nil binding view after bundle delete cleanup")
	}

	var bindingCount int64
	if err := db.Model(&model.ReferralTemplateBinding{}).
		Where("user_id = ? AND referral_type = ?", user.Id, model.ReferralTypeSubscription).
		Count(&bindingCount).Error; err != nil {
		t.Fatalf("failed to count bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("binding count = %d, want 0 after bundle delete cleanup", bindingCount)
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

func TestAdminListReferralTemplateBindingsBundleViewAggregatesByTemplateBundle(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "binding-bundle-list-user", 0, dto.UserSetting{})
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "binding-bundle-list-template",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 600,
	}, 1)
	if err != nil {
		t.Fatalf("failed to create template bundle: %v", err)
	}

	if _, err := model.UpsertReferralTemplateBindingBundleForUser(user.Id, model.ReferralTypeSubscription, rows[0].Id, nil, 1); err != nil {
		t.Fatalf("failed to create bundle binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/bindings/users/"+strconv.Itoa(user.Id)+"?referral_type="+model.ReferralTypeSubscription+"&view=bundle", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	ctx.Request.URL.RawQuery = "referral_type=" + model.ReferralTypeSubscription + "&view=bundle"
	AdminListReferralTemplateBindingsByUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var payload struct {
		Items []model.ReferralTemplateBindingBundleView `json:"items"`
	}
	if err := common.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("bundle item count = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].Name != "binding-bundle-list-template" {
		t.Fatalf("bundle name = %q, want binding-bundle-list-template", payload.Items[0].Name)
	}
	if len(payload.Items[0].BindingIDs) != 2 {
		t.Fatalf("binding id count = %d, want 2", len(payload.Items[0].BindingIDs))
	}
	if len(payload.Items[0].TemplateIDs) != 2 {
		t.Fatalf("template id count = %d, want 2", len(payload.Items[0].TemplateIDs))
	}
	if strings.Join(payload.Items[0].Groups, ",") != "default,vip" {
		t.Fatalf("groups = %v, want [default vip]", payload.Items[0].Groups)
	}
}

func TestAdminListReferralTemplateBindingsBundleViewShowsOnlyActuallyBoundGroups(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "binding-bundle-partial-user", 0, dto.UserSetting{})
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "binding-bundle-partial-template",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 600,
	}, 1)
	if err != nil {
		t.Fatalf("failed to create template bundle: %v", err)
	}

	if _, err := model.UpsertReferralTemplateBinding(&model.ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: model.ReferralTypeSubscription,
		TemplateId:   rows[0].Id,
		CreatedBy:    1,
		UpdatedBy:    1,
	}); err != nil {
		t.Fatalf("failed to create partial bundle binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/bindings/users/"+strconv.Itoa(user.Id)+"?referral_type="+model.ReferralTypeSubscription+"&view=bundle", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	ctx.Request.URL.RawQuery = "referral_type=" + model.ReferralTypeSubscription + "&view=bundle"
	AdminListReferralTemplateBindingsByUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var payload struct {
		Items []model.ReferralTemplateBindingBundleView `json:"items"`
	}
	if err := common.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("bundle item count = %d, want 1", len(payload.Items))
	}
	if strings.Join(payload.Items[0].Groups, ",") != "default" {
		t.Fatalf("groups = %v, want [default] for partial binding", payload.Items[0].Groups)
	}
	if len(payload.Items[0].BindingIDs) != 1 {
		t.Fatalf("binding id count = %d, want 1", len(payload.Items[0].BindingIDs))
	}
	if len(payload.Items[0].TemplateIDs) != 1 || payload.Items[0].TemplateIDs[0] != rows[0].Id {
		t.Fatalf("template ids = %v, want [%d]", payload.Items[0].TemplateIDs, rows[0].Id)
	}
}

func TestAdminUpsertReferralTemplateBindingUsesTemplateBundleScope(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "binding-admin-user", 0, dto.UserSetting{})
	oldRows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"default", "vip"},
		Name:                   "binding-admin-old-template",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 600,
	}, 1)
	if err != nil {
		t.Fatalf("failed to create old template bundle: %v", err)
	}
	newRows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           model.ReferralTypeSubscription,
		Groups:                 []string{"premium"},
		Name:                   "binding-admin-new-template",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1500,
		InviteeShareDefaultBps: 700,
	}, 1)
	if err != nil {
		t.Fatalf("failed to create new template bundle: %v", err)
	}

	oldBindings, err := model.UpsertReferralTemplateBindingBundleForUser(user.Id, model.ReferralTypeSubscription, oldRows[0].Id, nil, 1)
	if err != nil {
		t.Fatalf("failed to create old bundle binding: %v", err)
	}

	replaceBindingIDs := make([]int, 0, len(oldBindings))
	for _, binding := range oldBindings {
		replaceBindingIDs = append(replaceBindingIDs, binding.Id)
	}

	body := map[string]interface{}{
		"template_id":         newRows[0].Id,
		"replace_binding_ids": replaceBindingIDs,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/bindings/users/"+strconv.Itoa(user.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminUpsertReferralTemplateBindingForUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	defaultView, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, "default")
	if err != nil {
		t.Fatalf("failed to reload default binding view: %v", err)
	}
	if defaultView != nil {
		t.Fatalf("expected default binding to be replaced, got %+v", *defaultView)
	}

	vipView, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("failed to reload vip binding view: %v", err)
	}
	if vipView != nil {
		t.Fatalf("expected vip binding to be replaced, got %+v", *vipView)
	}

	premiumView, err := model.GetReferralTemplateBindingViewByUserAndScope(user.Id, model.ReferralTypeSubscription, "premium")
	if err != nil {
		t.Fatalf("failed to reload premium binding view: %v", err)
	}
	if premiumView == nil {
		t.Fatal("expected premium binding view")
	}
	if premiumView.Binding.TemplateId != newRows[0].Id {
		t.Fatalf("TemplateId = %d, want %d", premiumView.Binding.TemplateId, newRows[0].Id)
	}
}
