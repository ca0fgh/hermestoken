package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserSettingJSONOmitsUnsetQuotaTopupToggleButKeepsExplicitFalse(t *testing.T) {
	unsetPayload, err := json.Marshal(UserSetting{})
	if err != nil {
		t.Fatalf("marshal unset settings: %v", err)
	}
	if strings.Contains(string(unsetPayload), "quota_topup_enabled") {
		t.Fatalf("did not expect unset payload to include quota_topup_enabled: %s", string(unsetPayload))
	}

	disabled := false
	disabledPayload, err := json.Marshal(UserSetting{QuotaTopupEnabled: &disabled})
	if err != nil {
		t.Fatalf("marshal disabled settings: %v", err)
	}
	if !strings.Contains(string(disabledPayload), `"quota_topup_enabled":false`) {
		t.Fatalf("expected explicit false payload to keep quota_topup_enabled, got %s", string(disabledPayload))
	}
}
