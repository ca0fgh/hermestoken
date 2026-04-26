package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoDisabledChannelRecoveryCooldownMinutes float64 `json:"auto_disabled_channel_recovery_cooldown_minutes"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoDisabledChannelRecoveryCooldownMinutes: 30,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_RECOVERY_COOLDOWN_MINUTES") != "" {
		cooldownMinutes, err := strconv.ParseFloat(os.Getenv("CHANNEL_RECOVERY_COOLDOWN_MINUTES"), 64)
		if err == nil && cooldownMinutes >= 0 {
			monitorSetting.AutoDisabledChannelRecoveryCooldownMinutes = cooldownMinutes
		}
	}
	return &monitorSetting
}
