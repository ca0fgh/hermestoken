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

	var data struct {
		Items []struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Group    string `json:"group"`
		} `json:"items"`
		InviteeCount           int64 `json:"invitee_count"`
		TotalContributionQuota int64 `json:"total_contribution_quota"`
		Page                   int   `json:"page"`
		PageSize               int   `json:"page_size"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}

	if data.Page != 2 {
		t.Fatalf("page = %d, want 2", data.Page)
	}
	if data.PageSize != 1 {
		t.Fatalf("page_size = %d, want 1", data.PageSize)
	}
	if data.InviteeCount != 2 {
		t.Fatalf("invitee_count = %d, want 2", data.InviteeCount)
	}
	if data.TotalContributionQuota != 0 {
		t.Fatalf("total_contribution_quota = %d, want 0", data.TotalContributionQuota)
	}
	if len(data.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(data.Items))
	}
	if data.Items[0].ID != secondOwnedInvitee.Id {
		t.Fatalf("item id = %d, want %d", data.Items[0].ID, secondOwnedInvitee.Id)
	}
	if data.Items[0].Username != secondOwnedInvitee.Username {
		t.Fatalf("item username = %q, want %q", data.Items[0].Username, secondOwnedInvitee.Username)
	}
	if data.Items[0].Group != "vip" {
		t.Fatalf("item group = %q, want vip", data.Items[0].Group)
	}
	if data.Items[0].ID == foreignInvitee.Id {
		t.Fatalf("unexpected foreign invitee %d in response", foreignInvitee.Id)
	}
}

func TestGetSubscriptionReferralInviteeIncludesInviteeGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-detail-owner", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-detail-user", inviter.Id, dto.UserSetting{})
	if err := model.DB.Model(&model.User{}).Where("id = ?", invitee.Id).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update invitee group: %v", err)
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
		Invitee struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Group    string `json:"group"`
		} `json:"invitee"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if data.Invitee.ID != invitee.Id {
		t.Fatalf("invitee.id = %d, want %d", data.Invitee.ID, invitee.Id)
	}
	if data.Invitee.Username != invitee.Username {
		t.Fatalf("invitee.username = %q, want %q", data.Invitee.Username, invitee.Username)
	}
	if data.Invitee.Group != "vip" {
		t.Fatalf("invitee.group = %q, want vip", data.Invitee.Group)
	}
}

func TestGetSubscriptionReferralInviteeIncludesEnabledRatioSettingGroupsWithoutInviteeOverride(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true
	common.SubscriptionReferralGlobalRateBps = 2000
	setSubscriptionReferralGroupRatesForTest(t, `{"vip":4500}`)

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-detail-ratio-owner", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 900},
	})
	invitee := seedSubscriptionReferralControllerUser(t, "invitee-detail-ratio-user", inviter.Id, dto.UserSetting{})
	if _, err := model.UpsertSubscriptionReferralOverride(inviter.Id, "vip", 4500, 1); err != nil {
		t.Fatalf("failed to create inviter vip override: %v", err)
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
		AvailableGroups              []string       `json:"available_groups"`
		DefaultInviteeRateBpsByGroup map[string]int `json:"default_invitee_rate_bps_by_group"`
		EffectiveTotalRateBpsByGroup map[string]int `json:"effective_total_rate_bps_by_group"`
		Overrides                    []struct {
			Group string `json:"group"`
		} `json:"overrides"`
	}
	if err := common.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if len(data.Overrides) != 0 {
		t.Fatalf("expected no invitee-specific overrides, got %+v", data.Overrides)
	}
	foundVIP := false
	for _, group := range data.AvailableGroups {
		if group == "vip" {
			foundVIP = true
			break
		}
	}
	if !foundVIP {
		t.Fatalf("available_groups = %v, want vip included", data.AvailableGroups)
	}
	if data.DefaultInviteeRateBpsByGroup["vip"] != 900 {
		t.Fatalf("default_invitee_rate_bps_by_group[vip] = %d, want 900", data.DefaultInviteeRateBpsByGroup["vip"])
	}
	if data.EffectiveTotalRateBpsByGroup["vip"] != 4500 {
		t.Fatalf("effective_total_rate_bps_by_group[vip] = %d, want 4500", data.EffectiveTotalRateBpsByGroup["vip"])
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

func TestGetSubscriptionReferralInviteeHidesNonexistentInviteeIDs(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-missing-owner", 0, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees/9999",
		nil,
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: "9999"}}
	GetSubscriptionReferralInvitee(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected nonexistent invitee lookup to fail")
	}
	if resp.Message != "被邀请人不存在" {
		t.Fatalf("message = %q, want %q", resp.Message, "被邀请人不存在")
	}
}

func TestGetSubscriptionReferralInviteeSurfacesRealLookupErrors(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	ensureSubscriptionReferralInviteeOverrideTable(t)
	common.SubscriptionReferralEnabled = true

	inviter := seedSubscriptionReferralControllerUser(t, "invitee-db-error-owner", 0, dto.UserSetting{})
	sqlDB, err := model.DB.DB()
	if err != nil {
		t.Fatalf("failed to access sql db handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close sql db handle: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/referral/subscription/invitees/1",
		nil,
		inviter.Id,
	)
	ctx.Params = gin.Params{{Key: "invitee_id", Value: "1"}}
	GetSubscriptionReferralInvitee(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected invitee lookup to fail after database close")
	}
	if resp.Message == "被邀请人不存在" {
		t.Fatalf("expected real db error to surface, got normalized not-found message")
	}
}
