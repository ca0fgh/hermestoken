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

func TestGetSubscriptionReferralSelfReturnsEffectiveRates(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	_ = db
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000

	user := seedSubscriptionReferralControllerUser(t, "self-user", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBps: 500,
	})
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "", 3500, 1); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success")
	}

	var data struct {
		Enabled        bool `json:"enabled"`
		TotalRateBps   int  `json:"total_rate_bps"`
		InviteeRateBps int  `json:"invitee_rate_bps"`
		InviterRateBps int  `json:"inviter_rate_bps"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if !data.Enabled || data.TotalRateBps != 3500 || data.InviteeRateBps != 500 || data.InviterRateBps != 3000 {
		t.Fatalf("unexpected referral self payload: %+v", data)
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

func TestAdminUpsertSubscriptionReferralOverridePersistsOverride(t *testing.T) {
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
	if !resp.Success {
		t.Fatalf("expected success")
	}

	override, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "")
	if err != nil {
		t.Fatalf("failed to load override: %v", err)
	}
	if override.TotalRateBps != 3500 {
		t.Fatalf("expected persisted override 3500, got %d", override.TotalRateBps)
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
		HasOverride           bool `json:"has_override"`
		OverrideRateBps       int  `json:"override_rate_bps"`
		EffectiveTotalRateBps int  `json:"effective_total_rate_bps"`
	}
	if err := common.Unmarshal(getResp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if !data.HasOverride {
		t.Fatal("expected default-group legacy override to be reported")
	}
	if data.OverrideRateBps != 4100 {
		t.Fatalf("override_rate_bps = %d, want 4100", data.OverrideRateBps)
	}
	if data.EffectiveTotalRateBps != 4100 {
		t.Fatalf("effective_total_rate_bps = %d, want 4100", data.EffectiveTotalRateBps)
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
		AdminUpsertSubscriptionReferralOverrideRequest{TotalRateBps: 3500},
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
