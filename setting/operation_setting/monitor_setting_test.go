package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
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

func TestGetMonitorSetting_IgnoresLegacyChannelTestFrequencyEnv(t *testing.T) {
	original := monitorSetting
	t.Cleanup(func() {
		monitorSetting = original
	})
	t.Setenv("CHANNEL_TEST_FREQUENCY", "1")

	setting := GetMonitorSetting()
	exported := config.GlobalConfig.ExportAllConfigs()

	require.Equal(t, float64(30), setting.AutoDisabledChannelRecoveryCooldownMinutes)
	require.NotContains(t, exported, "monitor_setting.auto_test_channel_enabled")
	require.NotContains(t, exported, "monitor_setting.auto_test_channel_minutes")
}
