package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestUpdateUserRejectsReferralCycle(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	root := seedSubscriptionReferralControllerUser(t, "root-user", 0, dto.UserSetting{})
	root.Role = common.RoleRootUser
	if err := model.DB.Save(root).Error; err != nil {
		t.Fatalf("failed to promote root user: %v", err)
	}

	parent := seedSubscriptionReferralControllerUser(t, "parent-user", root.Id, dto.UserSetting{})
	child := seedSubscriptionReferralControllerUser(t, "child-user", parent.Id, dto.UserSetting{})

	body := map[string]interface{}{
		"id":         parent.Id,
		"username":   parent.Username,
		"password":   "",
		"display_name": parent.DisplayName,
		"group":      parent.Group,
		"quota":      parent.Quota,
		"role":       parent.Role,
		"status":     parent.Status,
		"inviter_id": child.Id,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", body, root.Id)
	ctx.Set("role", common.RoleRootUser)
	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected cycle validation failure, got success response: %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "cycle") {
		t.Fatalf("message = %q, want cycle validation error", response.Message)
	}
}
