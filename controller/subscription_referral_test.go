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

func TestGetSubscriptionReferralSelfReturnsTemplateBoundGroupsOnly(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "self-template-user", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, user.Id, "vip", model.ReferralLevelTypeDirect, 700)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var data struct {
		Enabled bool `json:"enabled"`
		Groups  []struct {
			Group          string `json:"group"`
			TemplateID     int    `json:"template_id"`
			LevelType      string `json:"level_type"`
			TotalRateBps   int    `json:"total_rate_bps"`
			InviteeRateBps int    `json:"invitee_rate_bps"`
			InviterRateBps int    `json:"inviter_rate_bps"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
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
	if group.LevelType != model.ReferralLevelTypeDirect {
		t.Fatalf("level_type = %q, want %q", group.LevelType, model.ReferralLevelTypeDirect)
	}
	if group.TotalRateBps != 1200 || group.InviteeRateBps != 700 || group.InviterRateBps != 500 {
		t.Fatalf("unexpected group payload: %+v", group)
	}
}

func TestGetSubscriptionReferralSelfIgnoresBindingDefaultOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	user := seedSubscriptionReferralControllerUser(t, "self-template-override-user", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, user.Id, "vip", model.ReferralLevelTypeDirect, 700)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var data struct {
		Groups []struct {
			Group          string `json:"group"`
			InviteeRateBps int    `json:"invitee_rate_bps"`
			InviterRateBps int    `json:"inviter_rate_bps"`
		} `json:"groups"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(data.Groups) != 1 {
		t.Fatalf("groups length = %d, want 1", len(data.Groups))
	}
	if data.Groups[0].Group != "vip" {
		t.Fatalf("group = %q, want vip", data.Groups[0].Group)
	}
	if data.Groups[0].InviteeRateBps != 700 || data.Groups[0].InviterRateBps != 500 {
		t.Fatalf("unexpected rates: %+v", data.Groups[0])
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
		t.Fatal("expected missing trade_no to fail")
	}
}

func TestGetSubscriptionReferralInviteeReturnsTemplateBoundGroupsOnly(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-template-owner", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-template-user", inviter.Id, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "vip", model.ReferralLevelTypeDirect, 700)

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(invitee.Id),
		nil,
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(invitee.Id)}}
	GetSubscriptionReferralInvitee(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var data struct {
		Scopes []struct {
			Group                   string `json:"group"`
			TemplateName            string `json:"template_name"`
			LevelType               string `json:"level_type"`
			TotalRateBps            int    `json:"total_rate_bps"`
			DefaultInviteeRateBps   int    `json:"default_invitee_rate_bps"`
			EffectiveInviteeRateBps int    `json:"effective_invitee_rate_bps"`
			HasOverride             bool   `json:"has_override"`
		} `json:"scopes"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(data.Scopes) != 1 {
		t.Fatalf("scopes length = %d, want 1", len(data.Scopes))
	}
	scope := data.Scopes[0]
	if scope.Group != "vip" {
		t.Fatalf("group = %q, want vip", scope.Group)
	}
	if scope.TemplateName != "vip-direct" {
		t.Fatalf("template_name = %q, want vip-direct", scope.TemplateName)
	}
	if scope.LevelType != model.ReferralLevelTypeDirect {
		t.Fatalf("level_type = %q, want %q", scope.LevelType, model.ReferralLevelTypeDirect)
	}
	if scope.TotalRateBps != 1200 {
		t.Fatalf("total_rate_bps = %d, want 1200", scope.TotalRateBps)
	}
	if scope.DefaultInviteeRateBps != 700 {
		t.Fatalf("default_invitee_rate_bps = %d, want 700", scope.DefaultInviteeRateBps)
	}
	if scope.EffectiveInviteeRateBps != 700 {
		t.Fatalf("effective_invitee_rate_bps = %d, want 700", scope.EffectiveInviteeRateBps)
	}
	if scope.HasOverride {
		t.Fatal("expected has_override to be false")
	}
}

func TestGetSubscriptionReferralInviteeReturnsContributionDetails(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	common.QuotaPerUnit = 100

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-detail-owner", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-detail-user", inviter.Id, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "vip", model.ReferralLevelTypeTeam, 700)

	plan := seedSubscriptionPlan(t, model.DB, "invitee-detail-plan")
	plan.UpgradeGroup = "vip"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	order := &model.SubscriptionOrder{
		UserId:        invitee.Id,
		PlanId:        plan.Id,
		Money:         10,
		TradeNo:       "invitee-detail-order",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := model.DB.Create(order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if err := model.CompleteSubscriptionOrder(order.TradeNo, `{\"ok\":true}`); err != nil {
		t.Fatalf("failed to complete order: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(invitee.Id),
		nil,
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(invitee.Id)}}
	GetSubscriptionReferralInvitee(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var data struct {
		ContributionDetails []struct {
			TradeNo              string `json:"trade_no"`
			Group                string `json:"group"`
			RewardComponent      string `json:"reward_component"`
			RoleType             string `json:"role_type"`
			EffectiveRewardQuota int64  `json:"effective_reward_quota"`
			Status               string `json:"status"`
		} `json:"contribution_details"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(data.ContributionDetails) != 2 {
		t.Fatalf("len(contribution_details) = %d, want 2", len(data.ContributionDetails))
	}
	if data.ContributionDetails[0].TradeNo != order.TradeNo {
		t.Fatalf("first trade_no = %q, want %q", data.ContributionDetails[0].TradeNo, order.TradeNo)
	}
	if data.ContributionDetails[0].Group != "vip" {
		t.Fatalf("first group = %q, want vip", data.ContributionDetails[0].Group)
	}
	if data.ContributionDetails[0].RewardComponent != "team_direct_reward" {
		t.Fatalf("first reward_component = %q, want team_direct_reward", data.ContributionDetails[0].RewardComponent)
	}
	if data.ContributionDetails[0].RoleType != model.ReferralLevelTypeTeam {
		t.Fatalf("first role_type = %q, want %q", data.ContributionDetails[0].RoleType, model.ReferralLevelTypeTeam)
	}
	if data.ContributionDetails[0].EffectiveRewardQuota <= 0 {
		t.Fatalf("first effective_reward_quota = %d, want > 0", data.ContributionDetails[0].EffectiveRewardQuota)
	}
	if data.ContributionDetails[1].RewardComponent != "invitee_reward" {
		t.Fatalf("second reward_component = %q, want invitee_reward", data.ContributionDetails[1].RewardComponent)
	}
}
