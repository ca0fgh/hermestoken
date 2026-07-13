package setting

import (
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTwoCryptoNetworks(t *testing.T) {
	t.Helper()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		"CryptoPaymentEnabled":     "true",
		"CryptoTronEnabled":        "true",
		"CryptoTronReceiveAddress": "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		"CryptoTronUSDTContract":   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		"CryptoBSCEnabled":         "true",
		"CryptoBSCReceiveAddress":  "0xaeaf7ECc0337c06911bc01C227135e3646B77580",
		"CryptoBSCUSDTContract":    "0x55d398326f99059fF775485246999027B3197955",
	}
	LoadCryptoPaymentSettingsFromOptionMap()
	t.Cleanup(func() {
		common.OptionMap = original
		LoadCryptoPaymentSettingsFromOptionMap()
		cryptoNetworkHealthMutex.Lock()
		cryptoNetworkHealth = map[string]CryptoNetworkHealth{}
		cryptoNetworkHealthMutex.Unlock()
	})
}

// The whole point of preflight: a network proven unable to collect money stops being
// offered. Production spent months handing out TRON deposit addresses for a contract
// that exists on no chain, and every one of those payments would have been lost.
func TestAProvenMisconfiguredNetworkIsNoLongerPayable(t *testing.T) {
	loadTwoCryptoNetworks(t)
	require.Len(t, GetEnabledCryptoPaymentNetworks(), 2)

	SetCryptoNetworkHealth(CryptoNetworkHealth{
		Network:   "tron_trc20",
		Verdict:   CryptoNetworkHealthMismatch,
		Detail:    "no contract is deployed at TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		CheckedAt: time.Now(),
	})

	payable := GetPayableCryptoPaymentNetworks()
	require.Len(t, payable, 1)
	assert.Equal(t, "bsc_erc20", payable[0].Network)
	assert.False(t, CryptoNetworkIsPayable("tron_trc20"))

	// The scanner must still be given the withdrawn network. A deposit already in
	// flight when the misconfiguration was caught has to keep being looked for, or
	// taking the network off sale would itself strand somebody's money.
	assert.Len(t, GetEnabledCryptoPaymentNetworks(), 2)
}

// An RPC outage costs time, not money — the deposit is matched later from the
// scanner cursor. Closing checkout over one would be a self-inflicted outage.
func TestAnUnverifiableNetworkStaysPayable(t *testing.T) {
	loadTwoCryptoNetworks(t)

	SetCryptoNetworkHealth(CryptoNetworkHealth{
		Network:   "tron_trc20",
		Verdict:   CryptoNetworkHealthUnknown,
		Detail:    "no RPC endpoint answered",
		CheckedAt: time.Now(),
	})

	assert.Len(t, GetPayableCryptoPaymentNetworks(), 2)
	assert.True(t, CryptoNetworkIsPayable("tron_trc20"))
}

// A network nobody has checked yet is payable, exactly as it was before preflight
// existed. Preflight may only ever take a network away, never be the reason one is
// missing at startup.
func TestAnUncheckedNetworkIsPayable(t *testing.T) {
	loadTwoCryptoNetworks(t)

	assert.Len(t, GetPayableCryptoPaymentNetworks(), 2)
}

func TestCryptoRPCEndpointsPutsTheConfiguredEndpointFirstAndKeepsAFallback(t *testing.T) {
	loadTwoCryptoNetworks(t)
	CryptoSolanaRPCURL = "https://rpc.ankr.com/solana/key"

	endpoints := CryptoRPCEndpoints("solana")

	assert.Contains(t, endpoints, "https://rpc.ankr.com/solana/key")
	// A single provider must never be the whole receive path: the Solana endpoint in
	// production was on devnet, and the key that owned it had no mainnet access at
	// all, so there was nowhere to fail over to.
	assert.Contains(t, endpoints, "https://api.mainnet-beta.solana.com")
	assert.True(t, len(endpoints) > len("https://rpc.ankr.com/solana/key"))
}
