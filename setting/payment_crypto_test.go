package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestGetCryptoPaymentNetworksHidesIncompleteNetworks(t *testing.T) {
	original := common.OptionMap
	common.OptionMap = map[string]string{
		"CryptoPaymentEnabled":     "true",
		"CryptoTronEnabled":        "true",
		"CryptoTronReceiveAddress": "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		"CryptoTronUSDTContract":   "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		"CryptoBSCEnabled":         "true",
		"CryptoBSCReceiveAddress":  "",
		"CryptoBSCUSDTContract":    "0x55d398326f99059fF775485246999027B3197955",
	}
	LoadCryptoPaymentSettingsFromOptionMap()
	t.Cleanup(func() {
		common.OptionMap = original
		LoadCryptoPaymentSettingsFromOptionMap()
	})

	networks := GetEnabledCryptoPaymentNetworks()
	assert.Len(t, networks, 1)
	assert.Equal(t, "tron_trc20", networks[0].Network)
	assert.Equal(t, 20, networks[0].Confirmations)
	assert.Equal(t, 10, CryptoOrderExpireMinutes)
}

func TestCryptoPaymentConfigValidation(t *testing.T) {
	assert.True(t, IsValidTronAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
	assert.False(t, IsValidTronAddress("0x1111111111111111111111111111111111111111"))
	assert.True(t, IsValidEVMAddress("0x55d398326f99059fF775485246999027B3197955"))
	assert.False(t, IsValidEVMAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
}
