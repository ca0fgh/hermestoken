package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestRegisterCreatesDefaultGroupUser(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})

	body := map[string]any{
		"username": "reg-default",
		"password": "password123",
		"group":    "",
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", body, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register response to succeed, got message: %s", response.Message)
	}

	var user model.User
	if err := db.Where("username = ?", "reg-default").First(&user).Error; err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}
	if user.Group != "default" {
		t.Fatalf("expected registered user group to be default, got %q", user.Group)
	}
}
