package controller

import (
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
)

func paymentGatewayToggleEnabled(key string, fallback bool) bool {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()

	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.EqualFold(value, "true")
}

func isEpayTopupSwitchEnabled() bool {
	return paymentGatewayToggleEnabled("EpayEnabled", true)
}

func isStripeTopupSwitchEnabled() bool {
	return paymentGatewayToggleEnabled("StripeEnabled", true)
}

func isCreemTopupSwitchEnabled() bool {
	return paymentGatewayToggleEnabled("CreemEnabled", true)
}

func isEpayTopupAvailable() bool {
	return isEpayTopupSwitchEnabled() &&
		operation_setting.PayAddress != "" &&
		operation_setting.EpayId != "" &&
		operation_setting.EpayKey != ""
}

func isStripeTopupAvailable() bool {
	return isStripeTopupSwitchEnabled() &&
		setting.StripeApiSecret != "" &&
		setting.StripeWebhookSecret != ""
}

func isCreemTopupAvailable() bool {
	return isCreemTopupSwitchEnabled() &&
		setting.CreemApiKey != "" &&
		setting.CreemProducts != "[]" &&
		(setting.CreemWebhookSecret != "" || setting.CreemTestMode)
}

func isWaffoTopupAvailable() bool {
	return setting.WaffoEnabled &&
		((!setting.WaffoSandbox &&
			setting.WaffoApiKey != "" &&
			setting.WaffoPrivateKey != "" &&
			setting.WaffoPublicCert != "") ||
			(setting.WaffoSandbox &&
				setting.WaffoSandboxApiKey != "" &&
				setting.WaffoSandboxPrivateKey != "" &&
				setting.WaffoSandboxPublicCert != ""))
}
