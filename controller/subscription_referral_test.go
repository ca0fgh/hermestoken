package controller

import (
	"encoding/json"
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
		SubscriptionReferralInviteeRateBps: 500,
	})
	invitee := seedSubscriptionReferralControllerUser(t, "admin-invitee", inviter.Id, dto.UserSetting{})
	plan := seedSubscriptionPlan(t, model.DB, "referral-admin-plan")
	plan.UpgradeGroup = "default"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update referral admin plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
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

func TestGetSubscriptionReferralSelfReturnsGroupedOnlyShape(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	setSubscriptionReferralGroupRatesForTest(t, `{"vip":4500}`)

	user := seedSubscriptionReferralControllerUser(t, "self-user-grouped-only", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 500},
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var payload map[string]json.RawMessage
	if err := common.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if _, exists := payload["total_rate_bps"]; exists {
		t.Fatal("expected total_rate_bps to be absent from self response")
	}
	if _, exists := payload["invitee_rate_bps"]; exists {
		t.Fatal("expected invitee_rate_bps to be absent from self response")
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
		t.Fatalf("failed to decode grouped self response: %v", err)
	}
	if !data.Enabled {
		t.Fatal("expected enabled to be true")
	}
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1 (%+v)", len(data.Groups), data.Groups)
	}
	group := data.Groups[0]
	if group.Group != "vip" {
		t.Fatalf("group = %q, want vip", group.Group)
	}
	if group.TotalRateBps != 4500 || group.InviteeRateBps != 500 || group.InviterRateBps != 4000 {
		t.Fatalf("unexpected grouped self payload: %+v", group)
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
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1 (%+v)", len(data.Groups), data.Groups)
	}
	if data.Groups[0].Group != "vip" {
		t.Fatalf("group = %q, want vip", data.Groups[0].Group)
	}
	if data.Groups[0].TotalRateBps != 2000 || data.Groups[0].InviteeRateBps != 0 || data.Groups[0].InviterRateBps != 2000 {
		t.Fatalf("unexpected plan-derived group payload: %+v", data.Groups[0])
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
		UpdateSubscriptionReferralSelfRequest{Group: "default", InviteeRateBps: 2100},
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

func TestAdminGetSubscriptionReferralSettingsReturnsGroupedRates(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"vip":3000}`)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/subscription/admin/referral/settings",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminGetSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var data struct {
		Enabled    bool           `json:"enabled"`
		GroupRates map[string]int `json:"group_rates"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if !data.Enabled {
		t.Fatal("expected enabled to be true")
	}
	if data.GroupRates["default"] != 4500 || data.GroupRates["vip"] != 3000 {
		t.Fatalf("unexpected group_rates payload: %+v", data.GroupRates)
	}
}

func TestAdminGetSubscriptionReferralSettingsIncludesPlanBackedGroups(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500}`)

	plan := seedSubscriptionPlan(t, model.DB, "settings-retired-group-plan")
	plan.UpgradeGroup = "retired"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update retired group plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/subscription/admin/referral/settings",
		nil,
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminGetSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var data struct {
		Groups []string `json:"groups"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if len(data.Groups) != 2 {
		t.Fatalf("groups length = %d, want 2 (%v)", len(data.Groups), data.Groups)
	}
	if data.Groups[0] != "default" || data.Groups[1] != "retired" {
		t.Fatalf("groups = %v, want [default retired]", data.Groups)
	}
}

func TestAdminUpdateSubscriptionReferralSettingsLegacyTotalRateBpsMapsToDefaultGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	initSubscriptionReferralOptionMapForTest()
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"vip":3000}`)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/settings",
		map[string]any{"total_rate_bps": 3600},
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminUpdateSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
	if got := common.GetSubscriptionReferralGroupRate("default"); got != 3600 {
		t.Fatalf("default group rate = %d, want 3600", got)
	}
	if got := common.GetSubscriptionReferralGroupRate("vip"); got != 3000 {
		t.Fatalf("vip group rate = %d, want 3000", got)
	}
}

func TestAdminUpdateSubscriptionReferralSettingsRejectsUnknownGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	initSubscriptionReferralOptionMapForTest()
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500}`)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/settings",
		map[string]any{"group_rates": map[string]int{"ghost": 3600}},
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminUpdateSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invalid group settings request to fail")
	}
	if got := common.GetSubscriptionReferralGroupRate("ghost"); got != 0 {
		t.Fatalf("ghost group rate = %d, want 0", got)
	}
}

func TestAdminUpdateSubscriptionReferralSettingsRejectsUnknownPositiveGroupWithoutUpdatingEnabled(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	initSubscriptionReferralOptionMapForTest()
	common.SubscriptionReferralEnabled = false
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500}`)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/settings",
		map[string]any{
			"enabled":     true,
			"group_rates": map[string]int{"ghost": 3600},
		},
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminUpdateSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invalid group settings request to fail")
	}
	if common.SubscriptionReferralEnabled {
		t.Fatal("expected SubscriptionReferralEnabled to remain false after rejected request")
	}
}

func TestAdminUpdateSubscriptionReferralSettingsAllowsRemovingStaleUnknownGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	initSubscriptionReferralOptionMapForTest()
	common.SubscriptionReferralEnabled = false
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"ghost":1200}`)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/subscription/admin/referral/settings",
		map[string]any{
			"enabled": true,
			"group_rates": map[string]int{
				"default": 4600,
				"ghost":   0,
			},
		},
		1,
	)
	ctx.Set("role", common.RoleRootUser)

	AdminUpdateSubscriptionReferralSettings(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected stale-group cleanup save to succeed, got message: %s", resp.Message)
	}
	if !common.SubscriptionReferralEnabled {
		t.Fatal("expected SubscriptionReferralEnabled to update after valid save")
	}
	if got := common.GetSubscriptionReferralGroupRate("default"); got != 4600 {
		t.Fatalf("default group rate = %d, want 4600", got)
	}
	if got := common.GetSubscriptionReferralGroupRate("ghost"); got != 0 {
		t.Fatalf("ghost group rate = %d, want 0 after cleanup", got)
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
		t.Fatalf("expected missing-group update to fail")
	}

	updatedUser, err := model.GetUserById(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	setting := updatedUser.GetSetting()
	if len(setting.SubscriptionReferralInviteeRateBpsByGroup) != 0 {
		t.Fatalf("expected grouped invitee rates to remain empty, got %+v", setting.SubscriptionReferralInviteeRateBpsByGroup)
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

func TestAdminUpsertSubscriptionReferralOverrideRejectsMissingGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user", 0, dto.UserSetting{})

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
		t.Fatalf("expected missing-group override upsert to fail")
	}

	var count int64
	if err := model.DB.Model(&model.SubscriptionReferralOverride{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count override rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("override row count = %d, want 0", count)
	}
}

func TestAdminGetSubscriptionReferralOverrideReturnsGroupedOnlyShape(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	setSubscriptionReferralGroupRatesForTest(t, `{"vip":4500}`)
	user := seedSubscriptionReferralControllerUser(t, "override-user-grouped-only-get", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 3500, 1); err != nil {
		t.Fatalf("failed to create grouped override: %v", err)
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

	var payload map[string]json.RawMessage
	if err := common.Unmarshal(getResp.Data, &payload); err != nil {
		t.Fatalf("failed to decode raw response data: %v", err)
	}
	if _, exists := payload["effective_total_rate_bps"]; exists {
		t.Fatal("expected effective_total_rate_bps to be absent from admin get override response")
	}
	if _, exists := payload["has_override"]; exists {
		t.Fatal("expected has_override to be absent from admin get override response")
	}

	var data struct {
		UserID int `json:"user_id"`
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
	if data.UserID != user.Id {
		t.Fatalf("user_id = %d, want %d", data.UserID, user.Id)
	}
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1 (%+v)", len(data.Groups), data.Groups)
	}
	group := data.Groups[0]
	if group.Group != "vip" {
		t.Fatalf("group = %q, want vip", group.Group)
	}
	if !group.HasOverride {
		t.Fatal("expected vip group to report override")
	}
	if group.OverrideRateBps == nil || *group.OverrideRateBps != 3500 {
		t.Fatalf("override_rate_bps = %v, want 3500", group.OverrideRateBps)
	}
	if group.EffectiveTotalRateBps != 3500 {
		t.Fatalf("effective_total_rate_bps = %d, want 3500", group.EffectiveTotalRateBps)
	}
}

func TestAdminGetSubscriptionReferralOverrideIgnoresLegacyEmptyGroupOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	setSubscriptionReferralGroupRatesForTest(t, `{"default":4500,"vip":3000}`)
	user := seedSubscriptionReferralControllerUser(t, "override-user-legacy-default-with-vip", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "", 4100, 1); err != nil {
		t.Fatalf("failed to create legacy empty-group override: %v", err)
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

	var foundDefault bool
	var defaultHasOverride bool
	var defaultOverrideRate *int
	var defaultEffectiveTotal int
	var foundVIP bool
	var vipHasOverride bool
	var vipOverrideRate *int
	var vipEffectiveTotal int
	for i := range data.Groups {
		switch data.Groups[i].Group {
		case "default":
			foundDefault = true
			defaultHasOverride = data.Groups[i].HasOverride
			defaultOverrideRate = data.Groups[i].OverrideRateBps
			defaultEffectiveTotal = data.Groups[i].EffectiveTotalRateBps
		case "vip":
			foundVIP = true
			vipHasOverride = data.Groups[i].HasOverride
			vipOverrideRate = data.Groups[i].OverrideRateBps
			vipEffectiveTotal = data.Groups[i].EffectiveTotalRateBps
		}
	}

	if !foundDefault {
		t.Fatalf("expected default group entry in %+v", data.Groups)
	}
	if defaultHasOverride {
		t.Fatal("expected legacy empty-group override to be ignored for default group")
	}
	if defaultOverrideRate != nil {
		t.Fatalf("expected nil default override rate, got %v", *defaultOverrideRate)
	}
	if defaultEffectiveTotal != 4500 {
		t.Fatalf("default effective_total_rate_bps = %d, want 4500", defaultEffectiveTotal)
	}
	if !foundVIP {
		t.Fatalf("expected vip group entry in %+v", data.Groups)
	}
	if vipHasOverride {
		t.Fatal("expected vip group to have no override")
	}
	if vipOverrideRate != nil {
		t.Fatalf("expected nil vip override rate, got %v", *vipOverrideRate)
	}
	if vipEffectiveTotal <= 0 {
		t.Fatalf("expected vip effective_total_rate_bps to be positive, got %d", vipEffectiveTotal)
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
	if data.Groups[0].EffectiveTotalRateBps != 2000 {
		t.Fatalf("vip effective_total_rate_bps = %d, want 2000", data.Groups[0].EffectiveTotalRateBps)
	}
	if data.Groups[0].HasOverride {
		t.Fatal("expected plan-derived group without override to report has_override=false")
	}
}

func TestAdminUpsertSubscriptionReferralOverrideRejectsMissingGroupWithoutMutatingExistingOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-default-upsert", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "default", 4100, 1); err != nil {
		t.Fatalf("failed to create default-group override: %v", err)
	}

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
		t.Fatalf("expected missing-group override update to fail")
	}

	defaultOverride, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "default")
	if err != nil {
		t.Fatalf("failed to load default-group override: %v", err)
	}
	if defaultOverride.TotalRateBps != 4100 {
		t.Fatalf("default override TotalRateBps = %d, want 4100", defaultOverride.TotalRateBps)
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

func TestAdminDeleteSubscriptionReferralOverridePreservesGroupedOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "override-user-delete", 0, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "", 3500, 1); err != nil {
		t.Fatalf("failed to create legacy ungrouped override: %v", err)
	}
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 2800, 1); err != nil {
		t.Fatalf("failed to create grouped override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/referral/users/1",
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

	if _, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, ""); err == nil {
		t.Fatal("expected legacy ungrouped override to be deleted")
	}
	groupedOverride, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("expected grouped override to remain: %v", err)
	}
	if groupedOverride.TotalRateBps != 2800 {
		t.Fatalf("grouped override TotalRateBps = %d, want 2800", groupedOverride.TotalRateBps)
	}

	var data struct {
		HasOverride           bool `json:"has_override"`
		EffectiveTotalRateBps int  `json:"effective_total_rate_bps"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if data.HasOverride {
		t.Fatal("expected legacy endpoint to report no remaining override")
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
		"/api/subscription/admin/referral/users/1",
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
