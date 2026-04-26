package controller

import (
	"net/http"
	"testing"
)

func TestUpdateOptionRejectsDeprecatedMonitorAutoTestOptions(t *testing.T) {
	for _, key := range []string{
		"monitor_setting.auto_test_channel_enabled",
		"monitor_setting.auto_test_channel_minutes",
	} {
		t.Run(key, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
				"key":   key,
				"value": "true",
			}, 1)

			UpdateOption(ctx)

			response := decodeAPIResponse(t, recorder)
			if response.Success {
				t.Fatalf("expected deprecated option %q to be rejected", key)
			}
		})
	}
}
