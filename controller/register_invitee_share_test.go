package controller

import (
	"net/http"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
)

func TestGetAffCodeReturnsSignedInviteeSharePayload(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	originalSessionSecret := common.SessionSecret
	common.SessionSecret = "aff-code-invitee-share-test-secret"
	t.Cleanup(func() {
		common.SessionSecret = originalSessionSecret
	})

	inviter := seedSubscriptionReferralControllerUser(t, "aff-code-owner", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/aff?invitee_rate_bps=450", nil, inviter.Id)
	ctx.Request.URL.RawQuery = "invitee_rate_bps=450"
	GetAffCode(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected aff response to succeed, got message: %s", response.Message)
	}

	var data struct {
		AffCode        string `json:"aff_code"`
		InviteeRateBps int    `json:"invitee_rate_bps"`
		InviteeRateSig string `json:"invitee_rate_sig"`
	}
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode aff response data: %v", err)
	}
	if data.AffCode != inviter.AffCode {
		t.Fatalf("aff_code = %q, want %q", data.AffCode, inviter.AffCode)
	}
	if data.InviteeRateBps != 450 {
		t.Fatalf("invitee_rate_bps = %d, want 450", data.InviteeRateBps)
	}
	if !model.ValidateReferralInviteeShareLinkSignature(data.AffCode, data.InviteeRateBps, data.InviteeRateSig) {
		t.Fatal("expected invitee share signature to validate")
	}
}

func TestRegisterWithSignedInviteeShareCreatesInviteeOverride(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalSessionSecret := common.SessionSecret

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	common.SessionSecret = "register-invitee-share-test-secret"

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		common.SessionSecret = originalSessionSecret
	})

	inviter := seedSubscriptionReferralControllerUser(t, "reg-share-inviter", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "default", model.ReferralLevelTypeDirect, 300)
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "vip", model.ReferralLevelTypeDirect, 500)

	signedRate := model.NewReferralInviteeShareLinkSignature(inviter.AffCode, 700)
	body := map[string]any{
		"username":             "reg-share-invitee",
		"password":             "password123",
		"aff_code":             inviter.AffCode,
		"invitee_rate_bps":     700,
		"invitee_rate_sig":     signedRate,
		"invitee_share_bps":    10000,
		"invitee_share_sig":    model.NewReferralInviteeShareLinkSignature(inviter.AffCode, 10000),
		"subscription_rebate":  9999,
		"subscription_rebate2": "ignored",
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", body, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register response to succeed, got message: %s", response.Message)
	}

	var invitee model.User
	if err := db.Where("username = ?", "reg-share-invitee").First(&invitee).Error; err != nil {
		t.Fatalf("failed to load registered invitee: %v", err)
	}
	if invitee.InviterId != inviter.Id {
		t.Fatalf("invitee inviter_id = %d, want %d", invitee.InviterId, inviter.Id)
	}

	overrides, err := model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list invitee share overrides: %v", err)
	}
	if len(overrides) != 2 {
		t.Fatalf("override count = %d, want 2: %+v", len(overrides), overrides)
	}
	for _, override := range overrides {
		if override.InviteeShareBps != 700 {
			t.Fatalf("override for group %q has bps %d, want 700", override.Group, override.InviteeShareBps)
		}
	}
}

func TestRegisterWithTamperedInviteeShareDoesNotCreateOverride(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalSessionSecret := common.SessionSecret

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	common.SessionSecret = "register-invitee-share-test-secret"

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		common.SessionSecret = originalSessionSecret
	})

	inviter := seedSubscriptionReferralControllerUser(t, "reg-share-tamper-inviter", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "default", model.ReferralLevelTypeDirect, 300)

	body := map[string]any{
		"username":         "reg-share-tamper",
		"password":         "password123",
		"aff_code":         inviter.AffCode,
		"invitee_rate_bps": 800,
		"invitee_rate_sig": model.NewReferralInviteeShareLinkSignature(inviter.AffCode, 300),
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", body, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register response to succeed, got message: %s", response.Message)
	}

	var invitee model.User
	if err := db.Where("username = ?", "reg-share-tamper").First(&invitee).Error; err != nil {
		t.Fatalf("failed to load registered invitee: %v", err)
	}

	overrides, err := model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list invitee share overrides: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("expected no overrides for tampered signature, got %+v", overrides)
	}
}

func TestRegisterWithOutOfRangeInviteeShareDoesNotNormalizeIntoSignedOverride(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalSessionSecret := common.SessionSecret

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	common.SessionSecret = "register-invitee-share-test-secret"

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		common.SessionSecret = originalSessionSecret
	})

	inviter := seedSubscriptionReferralControllerUser(t, "reg-share-out-of-range-inviter", 0, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "default", model.ReferralLevelTypeDirect, 300)

	body := map[string]any{
		"username":         "reg-share-range",
		"password":         "password123",
		"aff_code":         inviter.AffCode,
		"invitee_rate_bps": 20000,
		"invitee_rate_sig": model.NewReferralInviteeShareLinkSignature(inviter.AffCode, 10000),
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", body, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register response to succeed, got message: %s", response.Message)
	}

	var invitee model.User
	if err := db.Where("username = ?", "reg-share-range").First(&invitee).Error; err != nil {
		t.Fatalf("failed to load registered invitee: %v", err)
	}

	overrides, err := model.ListReferralInviteeShareOverrides(inviter.Id, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("failed to list invitee share overrides: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("expected no overrides for out-of-range signed rate, got %+v", overrides)
	}
}
