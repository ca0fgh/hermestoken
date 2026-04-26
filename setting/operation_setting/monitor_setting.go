package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled                     bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes                     float64 `json:"auto_test_channel_minutes"`
	AutoDisabledChannelRecoveryCooldownMinutes float64 `json:"auto_disabled_channel_recovery_cooldown_minutes"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:                     false,
	AutoTestChannelMinutes:                     10,
	AutoDisabledChannelRecoveryCooldownMinutes: 30,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	if os.Getenv("CHANNEL_RECOVERY_COOLDOWN_MINUTES") != "" {
		cooldownMinutes, err := strconv.ParseFloat(os.Getenv("CHANNEL_RECOVERY_COOLDOWN_MINUTES"), 64)
		if err == nil && cooldownMinutes >= 0 {
			monitorSetting.AutoDisabledChannelRecoveryCooldownMinutes = cooldownMinutes
		}
	}
	return &monitorSetting
}
