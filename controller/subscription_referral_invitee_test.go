package controller

import (
	"net/http"
	"strconv"
	"testing"

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

	overrides, err = model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list overrides after delete: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("overrides = %+v, want empty after delete", overrides)
	}
}
