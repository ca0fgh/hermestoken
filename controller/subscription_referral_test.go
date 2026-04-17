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
		AvailableGroups []string       `json:"available_groups"`
		DefaultInvitee  map[string]int `json:"default_invitee_rate_bps_by_group"`
		EffectiveTotal  map[string]int `json:"effective_total_rate_bps_by_group"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(data.AvailableGroups) != 1 || data.AvailableGroups[0] != "vip" {
		t.Fatalf("available_groups = %+v, want [vip]", data.AvailableGroups)
	}
	if data.DefaultInvitee["vip"] != 700 {
		t.Fatalf("default invitee rate = %d, want 700", data.DefaultInvitee["vip"])
	}
	if data.EffectiveTotal["vip"] != 1200 {
		t.Fatalf("effective total rate = %d, want 1200", data.EffectiveTotal["vip"])
	}
}
