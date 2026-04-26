package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetMonitorSetting_AutoDisabledRecoveryCooldownFromEnv(t *testing.T) {
	original := monitorSetting
	t.Cleanup(func() {
		monitorSetting = original
	})
	t.Setenv("CHANNEL_RECOVERY_COOLDOWN_MINUTES", "12.5")

	setting := GetMonitorSetting()

	require.Equal(t, 12.5, setting.AutoDisabledChannelRecoveryCooldownMinutes)
}
