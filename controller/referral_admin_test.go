package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestAdminCreateReferralTemplate(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.ReferralTemplate{}); err != nil {
		t.Fatalf("failed to migrate referral template: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"group":                     "vip",
		"name":                      "vip-direct",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1000,
		"team_cap_bps":              2500,
		"team_decay_ratio":          0.5,
		"team_max_depth":            3,
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

func TestAdminListLegacySubscriptionReferralSeeds(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate subscription referral invitee override: %v", err)
	}

	inviter := seedSubscriptionReferralControllerUser(t, "legacy_admin_seed_inviter", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 900},
	})
	invitee := seedSubscriptionReferralControllerUser(t, "legacy_admin_seed_invitee", inviter.Id, dto.UserSetting{})

	if _, err := model.UpsertSubscriptionReferralOverride(inviter.Id, "vip", 3200, inviter.Id); err != nil {
		t.Fatalf("failed to create legacy subscription override: %v", err)
	}
	if _, err := model.UpsertSubscriptionReferralInviteeOverride(inviter.Id, invitee.Id, "vip", 700); err != nil {
		t.Fatalf("failed to create legacy subscription invitee override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/legacy-seeds/subscription?group=vip", nil, inviter.Id)
	AdminListLegacySubscriptionReferralSeeds(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}
	if len(response.Data) == 0 {
		t.Fatal("expected non-empty legacy seed payload")
	}
}
