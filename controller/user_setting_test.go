package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestUpdateUserSettingPreservesExistingSettingFieldsAndSavesQuotaTopupToggle(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	user := &model.User{
		Id:       101,
		Username: "wallet-toggle-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Setting:  `{"language":"zh","sidebar_modules":"{\"personal\":{\"topup\":true}}","billing_preference":"wallet_first"}`,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := map[string]any{
		"notify_type":                          "email",
		"quota_warning_threshold":              100000.0,
		"quota_topup_enabled":                  false,
		"accept_unset_model_ratio_model":       true,
		"record_ip_log":                        true,
		"upstream_model_update_notify_enabled": false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", body, user.Id)

	UpdateUserSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	after, err := model.GetUserById(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	setting := after.GetSetting()

	if setting.QuotaTopupEnabled == nil || *setting.QuotaTopupEnabled {
		t.Fatalf("expected quota topup toggle to persist false, got %#v", setting.QuotaTopupEnabled)
	}
	if setting.Language != "zh" {
		t.Fatalf("expected language to be preserved, got %q", setting.Language)
	}
	if setting.BillingPreference != "wallet_first" {
		t.Fatalf("expected billing preference to be preserved, got %q", setting.BillingPreference)
	}
	if setting.SidebarModules == "" {
		t.Fatal("expected sidebar modules to be preserved")
	}
}
