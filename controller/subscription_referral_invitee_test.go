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

func TestGetSubscriptionReferralInviteesReturnsOnlyOwnedInvitees(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-list-owner", 0, dto.UserSetting{})
	firstOwnedInvitee := seedSubscriptionReferralControllerUser(t, "invitee-list-owned-1", inviter.Id, dto.UserSetting{})
	secondOwnedInvitee := seedSubscriptionReferralControllerUser(t, "invitee-list-owned-2", inviter.Id, dto.UserSetting{})
	otherInviter := seedSubscriptionReferralControllerUser(t, "invitee-list-other-owner", 0, dto.UserSetting{})
	foreignInvitee := seedSubscriptionReferralControllerUser(t, "invitee-list-foreign", otherInviter.Id, dto.UserSetting{})
	if err := model.DB.Model(&model.User{}).Where("id = ?", firstOwnedInvitee.Id).Update("group", "starter").Error; err != nil {
		t.Fatalf("failed to update first invitee group: %v", err)
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", secondOwnedInvitee.Id).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update second invitee group: %v", err)
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", foreignInvitee.Id).Update("group", "enterprise").Error; err != nil {
		t.Fatalf("failed to update foreign invitee group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees?page=2&page_size=1",
		nil,
		inviter.Id,
	)
	GetSubscriptionReferralInvitees(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
}

func TestGetSubscriptionReferralInviteesSearchesByGroupAndSortsByContributionQuota(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	common.QuotaPerUnit = 100

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-search-owner", 0, dto.UserSetting{})
	topInvitee := seedSubscriptionReferralControllerUser(t, "invitee-search-top", inviter.Id, dto.UserSetting{})
	lowInvitee := seedSubscriptionReferralControllerUser(t, "invitee-search-low", inviter.Id, dto.UserSetting{})

	seedActiveSubscriptionReferralBinding(t, inviter.Id, "default", model.ReferralLevelTypeDirect, 500)
	plan := seedSubscriptionPlan(t, model.DB, "invitee-search-plan")
	plan.UpgradeGroup = "default"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	createAndCompleteOrder := func(userID int, tradeNo string, money float64) {
		t.Helper()

		order := &model.SubscriptionOrder{
			UserId:        userID,
			PlanId:        plan.Id,
			Money:         money,
			TradeNo:       tradeNo,
			PaymentMethod: "epay",
			Status:        common.TopUpStatusPending,
			CreateTime:    common.GetTimestamp(),
		}
		if err := model.DB.Create(order).Error; err != nil {
			t.Fatalf("failed to create order %s: %v", tradeNo, err)
		}
		if err := model.CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
			t.Fatalf("failed to complete order %s: %v", tradeNo, err)
		}
	}

	createAndCompleteOrder(lowInvitee.Id, "invitee-search-order-low", 5)
	createAndCompleteOrder(topInvitee.Id, "invitee-search-order-top", 20)
	if err := model.DB.Model(&model.User{}).Where("id = ?", topInvitee.Id).Update("group", "svip").Error; err != nil {
		t.Fatalf("failed to update top invitee group: %v", err)
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", lowInvitee.Id).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update low invitee group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees?page=1&page_size=10",
		nil,
		inviter.Id,
	)
	GetSubscriptionReferralInvitees(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var listData struct {
		Items []struct {
			Id    int    `json:"id"`
			Group string `json:"group"`
		} `json:"items"`
	}
	if err := common.Unmarshal(resp.Data, &listData); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listData.Items) != 2 {
		t.Fatalf("items length = %d, want 2", len(listData.Items))
	}
	if listData.Items[0].Id != topInvitee.Id {
		t.Fatalf("first invitee id = %d, want %d", listData.Items[0].Id, topInvitee.Id)
	}

	searchCtx, searchRecorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees?keyword=svip&page=1&page_size=10",
		nil,
		inviter.Id,
	)
	searchCtx.Request.URL.RawQuery = "keyword=svip&page=1&page_size=10"
	GetSubscriptionReferralInvitees(searchCtx)

	searchResp := decodeAPIResponse(t, searchRecorder)
	if !searchResp.Success {
		t.Fatalf("expected group search success, got message: %s", searchResp.Message)
	}

	var searchData struct {
		Items []struct {
			Id    int    `json:"id"`
			Group string `json:"group"`
		} `json:"items"`
	}
	if err := common.Unmarshal(searchResp.Data, &searchData); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(searchData.Items) != 1 {
		t.Fatalf("searched items length = %d, want 1", len(searchData.Items))
	}
	if searchData.Items[0].Id != topInvitee.Id || searchData.Items[0].Group != "svip" {
		t.Fatalf("searched item = %+v, want top invitee in svip", searchData.Items[0])
	}

	idSearchCtx, idSearchRecorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees?keyword="+strconv.Itoa(lowInvitee.Id)+"&page=1&page_size=10",
		nil,
		inviter.Id,
	)
	idSearchCtx.Request.URL.RawQuery = "keyword=" + strconv.Itoa(lowInvitee.Id) + "&page=1&page_size=10"
	GetSubscriptionReferralInvitees(idSearchCtx)

	idSearchResp := decodeAPIResponse(t, idSearchRecorder)
	if !idSearchResp.Success {
		t.Fatalf("expected id search success, got message: %s", idSearchResp.Message)
	}

	var idSearchData struct {
		Items []struct {
			Id int `json:"id"`
		} `json:"items"`
	}
	if err := common.Unmarshal(idSearchResp.Data, &idSearchData); err != nil {
		t.Fatalf("failed to decode id search response: %v", err)
	}
	if len(idSearchData.Items) != 1 || idSearchData.Items[0].Id != lowInvitee.Id {
		t.Fatalf("id search items = %+v, want [%d]", idSearchData.Items, lowInvitee.Id)
	}
}

func TestUpsertSubscriptionReferralInviteeOverrideRequiresActiveBinding(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-no-binding-owner", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-no-binding-user", inviter.Id, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(invitee.Id),
		upsertSubscriptionReferralInviteeOverrideRequest{Group: "vip", InviteeRateBps: 500},
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(invitee.Id)}}
	UpsertSubscriptionReferralInviteeOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected missing binding request to fail")
	}
}

func TestUpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-override-owner", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "vip", model.ReferralLevelTypeDirect, 700)
	otherInviter := seedSubscriptionReferralControllerUser(t, "invitee-override-other-owner", 0, dto.UserSetting{})
	foreignInvitee := seedSubscriptionReferralControllerUser(t, "invitee-override-foreign", otherInviter.Id, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(foreignInvitee.Id),
		upsertSubscriptionReferralInviteeOverrideRequest{Group: "vip", InviteeRateBps: 500},
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(foreignInvitee.Id)}}
	UpsertSubscriptionReferralInviteeOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected foreign invitee update to fail")
	}
}

func TestUpsertAndDeleteSubscriptionReferralInviteeOverridePersistTemplateOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-template-owner", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-template-target", inviter.Id, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "vip", model.ReferralLevelTypeDirect, 700)

	updateCtx, updateRecorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(invitee.Id),
		upsertSubscriptionReferralInviteeOverrideRequest{Group: "vip", InviteeRateBps: 500},
		inviter.Id,
	)
	updateCtx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(invitee.Id)}}
	UpsertSubscriptionReferralInviteeOverride(updateCtx)

	updateResp := decodeAPIResponse(t, updateRecorder)
	if !updateResp.Success {
		t.Fatalf("expected success, got message: %s", updateResp.Message)
	}

	var updateData struct {
		Scopes []struct {
			Group                   string `json:"group"`
			OverrideInviteeRateBps  int    `json:"override_invitee_rate_bps"`
			EffectiveInviteeRateBps int    `json:"effective_invitee_rate_bps"`
			HasOverride             bool   `json:"has_override"`
		} `json:"scopes"`
	}
	if err := common.Unmarshal(updateResp.Data, &updateData); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if len(updateData.Scopes) != 1 {
		t.Fatalf("scopes length = %d, want 1", len(updateData.Scopes))
	}
	if !updateData.Scopes[0].HasOverride ||
		updateData.Scopes[0].Group != "vip" ||
		updateData.Scopes[0].OverrideInviteeRateBps != 500 ||
		updateData.Scopes[0].EffectiveInviteeRateBps != 500 {
		t.Fatalf("unexpected scope payload after update: %+v", updateData.Scopes[0])
	}

	overrides, err := model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list template invitee overrides: %v", err)
	}
	if len(overrides) != 1 || overrides[0].InviteeShareBps != 500 {
		t.Fatalf("overrides = %+v, want one row with 500 bps", overrides)
	}

	deleteCtx, deleteRecorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(invitee.Id)+"?group=vip",
		nil,
		inviter.Id,
	)
	deleteCtx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(invitee.Id)}}
	DeleteSubscriptionReferralInviteeOverride(deleteCtx)

	deleteResp := decodeAPIResponse(t, deleteRecorder)
	if !deleteResp.Success {
		t.Fatalf("expected delete success, got message: %s", deleteResp.Message)
	}

	var deleteData struct {
		Scopes []struct {
			Group                   string `json:"group"`
			EffectiveInviteeRateBps int    `json:"effective_invitee_rate_bps"`
			HasOverride             bool   `json:"has_override"`
		} `json:"scopes"`
	}
	if err := common.Unmarshal(deleteResp.Data, &deleteData); err != nil {
		t.Fatalf("failed to decode delete response: %v", err)
	}
	if len(deleteData.Scopes) != 1 {
		t.Fatalf("scopes length after delete = %d, want 1", len(deleteData.Scopes))
	}
	if deleteData.Scopes[0].HasOverride {
		t.Fatalf("expected override to be removed, got %+v", deleteData.Scopes[0])
	}
	if deleteData.Scopes[0].EffectiveInviteeRateBps != 700 {
		t.Fatalf("effective_invitee_rate_bps after delete = %d, want 700", deleteData.Scopes[0].EffectiveInviteeRateBps)
	}

	overrides, err = model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list overrides after delete: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("overrides = %+v, want empty after delete", overrides)
	}
}
