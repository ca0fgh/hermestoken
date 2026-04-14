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

func ensureSubscriptionReferralInviteeOverrideTable(t *testing.T) {
	t.Helper()

	if err := model.DB.AutoMigrate(&model.SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate invitee override table: %v", err)
	}
}

func TestGetSubscriptionReferralInviteesReturnsOnlyOwnedInvitees(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-list-owner", 0, dto.UserSetting{})
	ownedInvitee := seedSubscriptionReferralControllerUser(t, "invitee-list-owned", inviter.Id, dto.UserSetting{})
	otherInviter := seedSubscriptionReferralControllerUser(t, "invitee-list-other-owner", 0, dto.UserSetting{})
	foreignInvitee := seedSubscriptionReferralControllerUser(t, "invitee-list-foreign", otherInviter.Id, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees?page_size=10",
		nil,
		inviter.Id,
	)
	GetSubscriptionReferralInvitees(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}

	var data struct {
		Items []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"items"`
		InviteeCount           int64 `json:"invitee_count"`
		TotalContributionQuota int64 `json:"total_contribution_quota"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}

	if data.InviteeCount != 1 {
		t.Fatalf("invitee_count = %d, want 1", data.InviteeCount)
	}
	if data.TotalContributionQuota != 0 {
		t.Fatalf("total_contribution_quota = %d, want 0", data.TotalContributionQuota)
	}
	if len(data.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(data.Items))
	}
	if data.Items[0].ID != ownedInvitee.Id {
		t.Fatalf("item id = %d, want %d", data.Items[0].ID, ownedInvitee.Id)
	}
	if data.Items[0].Username != ownedInvitee.Username {
		t.Fatalf("item username = %q, want %q", data.Items[0].Username, ownedInvitee.Username)
	}
	if data.Items[0].ID == foreignInvitee.Id {
		t.Fatalf("unexpected foreign invitee %d in response", foreignInvitee.Id)
	}
}

func TestUpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true
	setSubscriptionReferralGroupRatesForTest(t, `{"vip":4500}`)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-override-owner", 0, dto.UserSetting{})
	if _, err := model.UpsertSubscriptionReferralOverride(inviter.Id, "vip", 4500, 1); err != nil {
		t.Fatalf("failed to create inviter vip override: %v", err)
	}
	otherInviter := seedSubscriptionReferralControllerUser(t, "invitee-override-other-owner", 0, dto.UserSetting{})
	foreignInvitee := seedSubscriptionReferralControllerUser(t, "invitee-override-foreign", otherInviter.Id, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription/invitees/"+strconv.Itoa(foreignInvitee.Id),
		map[string]any{"group": "vip", "invitee_rate_bps": 800},
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: strconv.Itoa(foreignInvitee.Id)}}
	UpsertSubscriptionReferralInviteeOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected foreign invitee override write to fail")
	}

	overrides, err := model.ListSubscriptionReferralInviteeOverrides(otherInviter.Id, foreignInvitee.Id)
	if err != nil {
		t.Fatalf("failed to load foreign invitee overrides: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("foreign invitee overrides length = %d, want 0", len(overrides))
	}
}
