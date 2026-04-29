package controller

import (
	"net/http"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
)

func TestGetSelfUsesLiveInviteeCountInsteadOfStoredAffCount(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	inviter := seedUser(t, db, 1, "default")
	if err := db.Model(&model.User{}).Where("id = ?", inviter.Id).Update("aff_count", 0).Error; err != nil {
		t.Fatalf("failed to reset inviter aff_count: %v", err)
	}

	invitee := &model.User{
		Username:  "self-live-invitee",
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   "self_live_invitee_code",
		InviterId: inviter.Id,
	}
	if err := db.Create(invitee).Error; err != nil {
		t.Fatalf("failed to create invitee: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, inviter.Id)
	ctx.Set("role", inviter.Role)
	GetSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var data struct {
		AffCount int64 `json:"aff_count"`
	}
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode self response: %v", err)
	}
	if data.AffCount != 1 {
		t.Fatalf("aff_count = %d, want 1", data.AffCount)
	}
}
