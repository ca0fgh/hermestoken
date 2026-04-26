package setting

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var (
	CryptoPaymentEnabled     = false
	CryptoTronEnabled        = false
	CryptoTronReceiveAddress = ""
	CryptoTronUSDTContract   = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	CryptoTronRPCURL         = ""
	CryptoTronAPIKey         = ""
	CryptoTronConfirmations  = 20
	CryptoBSCEnabled         = false
	CryptoBSCReceiveAddress  = ""
	CryptoBSCUSDTContract    = "0x55d398326f99059fF775485246999027B3197955"
	CryptoBSCRPCURL          = ""
	CryptoBSCConfirmations   = 15
	CryptoOrderExpireMinutes = 10
	CryptoUniqueSuffixMax    = 9999
	CryptoScannerEnabled     = true
)

type CryptoPaymentNetworkConfig struct {
	Network        string `json:"network"`
	DisplayName    string `json:"display_name"`
	Token          string `json:"token"`
	Contract       string `json:"contract"`
	ReceiveAddress string `json:"receive_address,omitempty"`
	Decimals       int    `json:"decimals"`
	Confirmations  int    `json:"confirmations"`
	MinTopUp       int    `json:"min_topup"`
}

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var tronAddressPattern = regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)

func IsValidEVMAddress(address string) bool {
	return evmAddressPattern.MatchString(strings.TrimSpace(address))
}

func IsValidTronAddress(address string) bool {
	return tronAddressPattern.MatchString(strings.TrimSpace(address))
}

func GetEnabledCryptoPaymentNetworks() []CryptoPaymentNetworkConfig {
	if !CryptoPaymentEnabled {
		return nil
	}
	networks := make([]CryptoPaymentNetworkConfig, 0, 2)
	if CryptoTronEnabled && IsValidTronAddress(CryptoTronReceiveAddress) && IsValidTronAddress(CryptoTronUSDTContract) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "tron_trc20",
			DisplayName:    "TRON TRC-20",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoTronUSDTContract),
			ReceiveAddress: strings.TrimSpace(CryptoTronReceiveAddress),
			Decimals:       6,
			Confirmations:  normalizedConfirmations(CryptoTronConfirmations, 20, 10),
			MinTopUp:       1,
		})
	}
	if CryptoBSCEnabled && IsValidEVMAddress(CryptoBSCReceiveAddress) && IsValidEVMAddress(CryptoBSCUSDTContract) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "bsc_erc20",
			DisplayName:    "BSC",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoBSCUSDTContract),
			ReceiveAddress: strings.TrimSpace(CryptoBSCReceiveAddress),
			Decimals:       18,
			Confirmations:  normalizedConfirmations(CryptoBSCConfirmations, 15, 8),
			MinTopUp:       1,
		})
	}
	return networks
}

func LoadCryptoPaymentSettingsFromOptionMap() {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	CryptoPaymentEnabled = optionBool("CryptoPaymentEnabled", false)
	CryptoTronEnabled = optionBool("CryptoTronEnabled", false)
	CryptoTronReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoTronReceiveAddress"])
	CryptoTronUSDTContract = optionString("CryptoTronUSDTContract", "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj")
	CryptoTronRPCURL = strings.TrimSpace(common.OptionMap["CryptoTronRPCURL"])
	CryptoTronAPIKey = strings.TrimSpace(common.OptionMap["CryptoTronAPIKey"])
	CryptoTronConfirmations = optionInt("CryptoTronConfirmations", 20)
	CryptoBSCEnabled = optionBool("CryptoBSCEnabled", false)
	CryptoBSCReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoBSCReceiveAddress"])
	CryptoBSCUSDTContract = optionString("CryptoBSCUSDTContract", "0x55d398326f99059fF775485246999027B3197955")
	CryptoBSCRPCURL = strings.TrimSpace(common.OptionMap["CryptoBSCRPCURL"])
	CryptoBSCConfirmations = optionInt("CryptoBSCConfirmations", 15)
	CryptoOrderExpireMinutes = optionInt("CryptoOrderExpireMinutes", 10)
	CryptoUniqueSuffixMax = optionInt("CryptoUniqueSuffixMax", 9999)
	CryptoScannerEnabled = optionBool("CryptoScannerEnabled", true)
}

func optionString(key string, fallback string) string {
	value := strings.TrimSpace(common.OptionMap[key])
	if value == "" {
		return fallback
	}
	return value
}

func optionInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(common.OptionMap[key]))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func optionBool(key string, fallback bool) bool {
	value := strings.TrimSpace(common.OptionMap[key])
	if value == "" {
		return fallback
	}
	return value == "true"
}

func normalizedConfirmations(value int, fallback int, minimum int) int {
	if value <= 0 {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	return value
}
