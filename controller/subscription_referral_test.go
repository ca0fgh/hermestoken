package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func setSubscriptionReferralGroupRatesForTest(t *testing.T, jsonStr string) {
	t.Helper()

	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(jsonStr); err != nil {
		t.Fatalf("UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
	}
	t.Cleanup(func() {
		if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup UpdateSubscriptionReferralGroupRatesByJSONString() error = %v", err)
		}
	})
}

func initSubscriptionReferralOptionMapForTest() {
	model.InitOptionMap()
}

func seedSubscriptionReferralControllerUser(t *testing.T, username string, inviterID int, setting dto.UserSetting) *model.User {
	t.Helper()

	user := &model.User{
		Username:  username,
		Password:  "password",
		AffCode:   username + "_code",
		Group:     "default",
		InviterId: inviterID,
	}
	user.SetSetting(setting)
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func seedSubscriptionReferralControllerTradeNo(t *testing.T) string {
	t.Helper()

	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	common.QuotaPerUnit = 100

	inviter := seedSubscriptionReferralControllerUser(t, "admin-inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"default": 500},
	})
	invitee := seedSubscriptionReferralControllerUser(t, "admin-invitee", inviter.Id, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "referral-admin-plan")
	plan.UpgradeGroup = "default"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update referral admin plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	if _, err := model.UpsertSubscriptionReferralOverride(inviter.Id, "default", 2000, 1); err != nil {
		t.Fatalf("failed to create referral admin override: %v", err)
	}
	order := &model.SubscriptionOrder{
		UserId:        invitee.Id,
		PlanId:        plan.Id,
		Money:         10,
		TradeNo:       "trade-ref-admin",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := model.DB.Create(order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if err := model.CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("failed to complete referral order: %v", err)
	}
	return order.TradeNo
}

func TestGetSubscriptionReferralSelfReturnsEffectiveRates(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	_ = db
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000

	user := seedSubscriptionReferralControllerUser(t, "self-user", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 500},
	})
	plan := seedSubscriptionPlan(t, model.DB, "self-user-vip-plan")
	plan.UpgradeGroup = "vip"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update self vip plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 3500, 1); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var data struct {
		Enabled bool `json:"enabled"`
		Groups  []struct {
			Group          string `json:"group"`
			TotalRateBps   int    `json:"total_rate_bps"`
			InviteeRateBps int    `json:"invitee_rate_bps"`
			InviterRateBps int    `json:"inviter_rate_bps"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if !data.Enabled || len(data.Groups) != 1 {
		t.Fatalf("unexpected referral self payload: %+v", data)
	}
	if data.Groups[0].Group != "vip" || data.Groups[0].TotalRateBps != 3500 || data.Groups[0].InviteeRateBps != 500 || data.Groups[0].InviterRateBps != 3000 {
		t.Fatalf("unexpected referral self payload: %+v", data)
	}
}

func TestGetSubscriptionReferralSelfIncludesPlanUpgradeGroupWithoutConfiguredRate(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{}`)

	user := seedSubscriptionReferralControllerUser(t, "self-user-plan-group", 0, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "self-group-plan")
	plan.UpgradeGroup = "vip"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update self group plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var data struct {
		Groups []struct {
			Group          string `json:"group"`
			TotalRateBps   int    `json:"total_rate_bps"`
			InviteeRateBps int    `json:"invitee_rate_bps"`
			InviterRateBps int    `json:"inviter_rate_bps"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if len(data.Groups) != 0 {
		t.Fatalf("expected no eligible referral groups without explicit user override, got %+v", data.Groups)
	}
}

func TestUpdateSubscriptionReferralSelfRejectsInviteeRateOverTotal(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000

	user := seedSubscriptionReferralControllerUser(t, "self-update-user", 0, dto.UserSetting{})
	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		UpdateSubscriptionReferralSelfRequest{InviteeRateBps: 2100},
		user.Id,
	)
	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatalf("expected validation failure")
	}
}

func TestUpdateSubscriptionReferralSelfAllowsPlanBackedRetiredGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{}`)

	user := seedSubscriptionReferralControllerUser(t, "self-update-plan-backed-group", 0, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "self-update-retired-group-plan")
	plan.UpgradeGroup = "retired"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update retired group plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "retired", 2600, 1); err != nil {
		t.Fatalf("failed to create retired group override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		UpdateSubscriptionReferralSelfRequest{Group: "retired", InviteeRateBps: 900},
		user.Id,
	)
	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected retired plan-backed group update to succeed, got message: %s", resp.Message)
	}

	updatedUser, err := model.GetUserById(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload updated user: %v", err)
	}
	if got := updatedUser.GetSetting().SubscriptionReferralInviteeRateBpsByGroup["retired"]; got != 900 {
		t.Fatalf("retired group invitee rate = %d, want 900", got)
	}
}

func TestAdminUpsertSubscriptionReferralOverridePersistsGroupSpecificOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-vip", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/1",
		map[string]any{"group": "vip", "total_rate_bps": 3500},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	override, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("failed to load vip override: %v", err)
	}
	if override.TotalRateBps != 3500 {
		t.Fatalf("vip override TotalRateBps = %d, want 3500", override.TotalRateBps)
	}
}

func TestAdminUpsertSubscriptionReferralOverrideAllowsPlanBackedRetiredGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-retired-group", 0, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "override-retired-group-plan")
	plan.UpgradeGroup = "retired"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update retired group plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/1",
		map[string]any{"group": "retired", "total_rate_bps": 3100},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected retired plan-backed override upsert to succeed, got message: %s", resp.Message)
	}

	override, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "retired")
	if err != nil {
		t.Fatalf("failed to load retired override: %v", err)
	}
	if override.TotalRateBps != 3100 {
		t.Fatalf("retired override TotalRateBps = %d, want 3100", override.TotalRateBps)
	}
}

func TestAdminUpsertSubscriptionReferralOverrideRejectsUnknownGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-invalid-group", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/1",
		map[string]any{"group": "ghost", "total_rate_bps": 3500},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invalid group override request to fail")
	}

	var count int64
	if err := model.DB.Model(&model.SubscriptionReferralOverride{}).Where("user_id = ? AND `group` = ?", user.Id, "ghost").Count(&count).Error; err != nil {
		t.Fatalf("failed to count ghost overrides: %v", err)
	}
	if count != 0 {
		t.Fatalf("ghost override count = %d, want 0", count)
	}
}

func TestUpdateSubscriptionReferralSelfStoresPerGroupInviteeRate(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"vip":4500}`)

	user := seedSubscriptionReferralControllerUser(t, "self-update-vip", 0, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "self-update-vip-plan")
	plan.UpgradeGroup = "vip"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update self update vip plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 4500, 1); err != nil {
		t.Fatalf("failed to create user vip referral override: %v", err)
	}
	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		map[string]any{"group": "vip", "invitee_rate_bps": 500},
		user.Id,
	)

	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	updatedUser, err := model.GetUserById(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if got := updatedUser.GetSetting().SubscriptionReferralInviteeRateBpsByGroup["vip"]; got != 500 {
		t.Fatalf("group invitee rate = %d, want 500", got)
	}
}

func TestUpdateSubscriptionReferralSelfRejectsMissingGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"vip":3000}`)

	user := seedSubscriptionReferralControllerUser(t, "self-update-missing-group", 0, dto.UserSetting{})
	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		map[string]any{"group": "", "invitee_rate_bps": 500},
		user.Id,
	)

	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected empty-group request to fail")
	}
}

func TestAdminUpsertSubscriptionReferralOverrideRejectsMissingGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-missing-group", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/1",
		AdminUpsertSubscriptionReferralOverrideRequest{TotalRateBps: 3500},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected missing-group override request to fail")
	}
}

func TestUpdateSubscriptionReferralSelfRejectsUnknownGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"vip":3000}`)

	user := seedSubscriptionReferralControllerUser(t, "self-update-invalid-group", 0, dto.UserSetting{})
	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		map[string]any{"group": "ghost", "invitee_rate_bps": 500},
		user.Id,
	)

	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invalid group request to fail")
	}
}

func TestAdminGetSubscriptionReferralOverrideReadsDefaultGroupLegacyOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-default-get", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "default", 4100, 1); err != nil {
		t.Fatalf("failed to create default-group override: %v", err)
	}

	getCtx, getRecorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/subscription/admin/referral/users/1",
		nil,
		1,
	)
	getCtx.Set("role", common.RoleRootUser)
	getCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminGetSubscriptionReferralOverride(getCtx)

	getResp := decodeAPIResponse(t, getRecorder)
	if !getResp.Success {
		t.Fatalf("expected admin get override success")
	}

	var data struct {
		Groups []struct {
			Group                 string `json:"group"`
			EffectiveTotalRateBps int    `json:"effective_total_rate_bps"`
			HasOverride           bool   `json:"has_override"`
			OverrideRateBps       *int   `json:"override_rate_bps"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(getResp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1 (%+v)", len(data.Groups), data.Groups)
	}
	if data.Groups[0].Group != "default" {
		t.Fatalf("group = %q, want default", data.Groups[0].Group)
	}
	if !data.Groups[0].HasOverride {
		t.Fatal("expected default-group override to be reported")
	}
	if data.Groups[0].OverrideRateBps == nil || *data.Groups[0].OverrideRateBps != 4100 {
		t.Fatalf("override_rate_bps = %v, want 4100", data.Groups[0].OverrideRateBps)
	}
	if data.Groups[0].EffectiveTotalRateBps != 4100 {
		t.Fatalf("effective_total_rate_bps = %d, want 4100", data.Groups[0].EffectiveTotalRateBps)
	}
}

func TestAdminGetSubscriptionReferralOverrideIncludesPlanUpgradeGroupWithoutConfiguredRate(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	setSubscriptionReferralGroupRatesForTest(t, `{}`)
	user := seedSubscriptionReferralControllerUser(t, "override-user-plan-group", 0, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "override-group-plan")
	plan.UpgradeGroup = "vip"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update override group plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	getCtx, getRecorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/subscription/admin/referral/users/1",
		nil,
		1,
	)
	getCtx.Set("role", common.RoleRootUser)
	getCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminGetSubscriptionReferralOverride(getCtx)

	getResp := decodeAPIResponse(t, getRecorder)
	if !getResp.Success {
		t.Fatalf("expected admin get override success")
	}

	var data struct {
		Groups []struct {
			Group                 string `json:"group"`
			EffectiveTotalRateBps int    `json:"effective_total_rate_bps"`
			HasOverride           bool   `json:"has_override"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(getResp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1 (%+v)", len(data.Groups), data.Groups)
	}
	if data.Groups[0].Group != "vip" {
		t.Fatalf("group = %q, want vip", data.Groups[0].Group)
	}
	if data.Groups[0].EffectiveTotalRateBps != 0 {
		t.Fatalf("vip effective_total_rate_bps = %d, want 0", data.Groups[0].EffectiveTotalRateBps)
	}
	if data.Groups[0].HasOverride {
		t.Fatal("expected plan-derived group without override to report has_override=false")
	}
}

func TestAdminUpsertSubscriptionReferralOverrideUpdatesDefaultGroupLegacyOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-default-upsert", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "default", 4100, 1); err != nil {
		t.Fatalf("failed to create default-group override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/1",
		AdminUpsertSubscriptionReferralOverrideRequest{Group: "default", TotalRateBps: 3500},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	defaultOverride, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default")
	if err != nil {
		t.Fatalf("failed to load default-group override: %v", err)
	}
	if defaultOverride.TotalRateBps != 3500 {
		t.Fatalf("default override TotalRateBps = %d, want 3500", defaultOverride.TotalRateBps)
	}
	if _, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, ""); err == nil {
		t.Fatal("expected no ungrouped override row to be created")
	}
}

func TestAdminUpsertSubscriptionReferralOverrideRejectsMissingUser(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/users/9999",
		AdminUpsertSubscriptionReferralOverrideRequest{TotalRateBps: 3500},
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "9999"}}
	AdminUpsertSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatalf("expected missing user override request to fail")
	}
}

func TestAdminDeleteSubscriptionReferralOverrideDeletesDefaultGroupLegacyOverrideOnly(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-default-delete", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "default", 3500, 1); err != nil {
		t.Fatalf("failed to create default-group override: %v", err)
	}
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 2800, 1); err != nil {
		t.Fatalf("failed to create grouped override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/referral/users/1?group=default",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminDeleteSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	if _, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default"); err == nil {
		t.Fatal("expected default-group legacy override to be deleted")
	}
	groupedOverride, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("expected grouped override to remain: %v", err)
	}
	if groupedOverride.TotalRateBps != 2800 {
		t.Fatalf("grouped override TotalRateBps = %d, want 2800", groupedOverride.TotalRateBps)
	}
	if _, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, ""); err == nil {
		t.Fatal("expected no ungrouped override row after delete")
	}
}

func TestAdminDeleteSubscriptionReferralOverrideRejectsUnknownGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-delete-invalid-group", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/referral/users/1?group=ghost",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminDeleteSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invalid group delete request to fail")
	}
}

func TestAdminDeleteSubscriptionReferralOverrideAllowsRetiredExistingGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-delete-retired-group", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "retired", 2800, 1); err != nil {
		t.Fatalf("failed to create retired grouped override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/referral/users/1?group=retired",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}

	AdminDeleteSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected retired override delete to succeed, got message: %s", resp.Message)
	}
	if _, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "retired"); err == nil {
		t.Fatal("expected retired grouped override to be deleted")
	}
}

func TestAdminReverseSubscriptionReferralIsIdempotent(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	tradeNo := seedSubscriptionReferralControllerTradeNo(t)

	for i := 0; i < 2; i++ {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/admin/referral/orders/"+tradeNo+"/reverse", nil, 1)
		ctx.Set("role", common.RoleRootUser)
		ctx.Params = gin.Params{{Key: "trade_no", Value: tradeNo}}
		AdminReverseSubscriptionReferral(ctx)

		resp := decodeAPIResponse(t, recorder)
		if !resp.Success {
			t.Fatalf("reverse call %d failed: %s", i+1, resp.Message)
		}
	}
}

func TestAdminReverseSubscriptionReferralRejectsUnknownTradeNo(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		"/api/subscription/admin/referral/orders/missing/reverse",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "trade_no", Value: "missing"}}
	AdminReverseSubscriptionReferral(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatalf("expected missing trade_no reversal to fail")
	}
}
