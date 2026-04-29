package setting

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
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

func TestGetCryptoPaymentNetworksIncludesPolygonAndSolana(t *testing.T) {
	original := common.OptionMap
	common.OptionMap = map[string]string{
		"CryptoPaymentEnabled":        "true",
		"CryptoPolygonEnabled":        "true",
		"CryptoPolygonReceiveAddress": "0x1111111111111111111111111111111111111111",
		"CryptoPolygonUSDTContract":   "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
		"CryptoPolygonConfirmations":  "128",
		"CryptoSolanaEnabled":         "true",
		"CryptoSolanaReceiveAddress":  "7YttLkHDoWJYNNe7U2s1owz8FC6xk4kZqGSPdU2ovbYW",
		"CryptoSolanaUSDTMint":        "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
		"CryptoSolanaConfirmations":   "32",
	}
	LoadCryptoPaymentSettingsFromOptionMap()
	t.Cleanup(func() {
		common.OptionMap = original
		LoadCryptoPaymentSettingsFromOptionMap()
	})

	networks := GetEnabledCryptoPaymentNetworks()
	assert.Len(t, networks, 2)
	assert.Equal(t, "polygon_pos", networks[0].Network)
	assert.Equal(t, "Polygon PoS", networks[0].DisplayName)
	assert.Equal(t, 6, networks[0].Decimals)
	assert.Equal(t, 128, networks[0].Confirmations)
	assert.Equal(t, "solana", networks[1].Network)
	assert.Equal(t, "Solana", networks[1].DisplayName)
	assert.Equal(t, 6, networks[1].Decimals)
	assert.Equal(t, 32, networks[1].Confirmations)
}

func TestCryptoPaymentConfigValidation(t *testing.T) {
	assert.True(t, IsValidTronAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
	assert.False(t, IsValidTronAddress("0x1111111111111111111111111111111111111111"))
	assert.True(t, IsValidEVMAddress("0x55d398326f99059fF775485246999027B3197955"))
	assert.False(t, IsValidEVMAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
	assert.True(t, IsValidSolanaAddress("Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"))
	assert.False(t, IsValidSolanaAddress("0x1111111111111111111111111111111111111111"))
}
