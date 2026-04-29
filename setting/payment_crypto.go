package setting

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

var (
	CryptoPaymentEnabled        = false
	CryptoTronEnabled           = false
	CryptoTronReceiveAddress    = ""
	CryptoTronUSDTContract      = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	CryptoTronRPCURL            = ""
	CryptoTronAPIKey            = ""
	CryptoTronConfirmations     = 20
	CryptoBSCEnabled            = false
	CryptoBSCReceiveAddress     = ""
	CryptoBSCUSDTContract       = "0x55d398326f99059fF775485246999027B3197955"
	CryptoBSCRPCURL             = ""
	CryptoBSCConfirmations      = 15
	CryptoPolygonEnabled        = false
	CryptoPolygonReceiveAddress = ""
	CryptoPolygonUSDTContract   = "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"
	CryptoPolygonRPCURL         = ""
	CryptoPolygonConfirmations  = 128
	CryptoSolanaEnabled         = false
	CryptoSolanaReceiveAddress  = ""
	CryptoSolanaUSDTMint        = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	CryptoSolanaRPCURL          = ""
	CryptoSolanaConfirmations   = 32
	CryptoOrderExpireMinutes    = 10
	CryptoUniqueSuffixMax       = 9999
	CryptoScannerEnabled        = true
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
var solanaAddressPattern = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

func IsValidEVMAddress(address string) bool {
	return evmAddressPattern.MatchString(strings.TrimSpace(address))
}

func IsValidTronAddress(address string) bool {
	return tronAddressPattern.MatchString(strings.TrimSpace(address))
}

func IsValidSolanaAddress(address string) bool {
	return solanaAddressPattern.MatchString(strings.TrimSpace(address))
}

func GetEnabledCryptoPaymentNetworks() []CryptoPaymentNetworkConfig {
	if !CryptoPaymentEnabled {
		return nil
	}
	networks := make([]CryptoPaymentNetworkConfig, 0, 4)
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
	if CryptoPolygonEnabled && IsValidEVMAddress(CryptoPolygonReceiveAddress) && IsValidEVMAddress(CryptoPolygonUSDTContract) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "polygon_pos",
			DisplayName:    "Polygon PoS",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoPolygonUSDTContract),
			ReceiveAddress: strings.TrimSpace(CryptoPolygonReceiveAddress),
			Decimals:       6,
			Confirmations:  normalizedConfirmations(CryptoPolygonConfirmations, 128, 32),
			MinTopUp:       1,
		})
	}
	if CryptoSolanaEnabled && IsValidSolanaAddress(CryptoSolanaReceiveAddress) && IsValidSolanaAddress(CryptoSolanaUSDTMint) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "solana",
			DisplayName:    "Solana",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoSolanaUSDTMint),
			ReceiveAddress: strings.TrimSpace(CryptoSolanaReceiveAddress),
			Decimals:       6,
			Confirmations:  normalizedConfirmations(CryptoSolanaConfirmations, 32, 1),
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
	CryptoPolygonEnabled = optionBool("CryptoPolygonEnabled", false)
	CryptoPolygonReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoPolygonReceiveAddress"])
	CryptoPolygonUSDTContract = optionString("CryptoPolygonUSDTContract", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F")
	CryptoPolygonRPCURL = strings.TrimSpace(common.OptionMap["CryptoPolygonRPCURL"])
	CryptoPolygonConfirmations = optionInt("CryptoPolygonConfirmations", 128)
	CryptoSolanaEnabled = optionBool("CryptoSolanaEnabled", false)
	CryptoSolanaReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoSolanaReceiveAddress"])
	CryptoSolanaUSDTMint = optionString("CryptoSolanaUSDTMint", "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")
	CryptoSolanaRPCURL = strings.TrimSpace(common.OptionMap["CryptoSolanaRPCURL"])
	CryptoSolanaConfirmations = optionInt("CryptoSolanaConfirmations", 32)
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
