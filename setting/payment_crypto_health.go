package setting

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// CryptoNetworkHealthOK — the RPC serves the chain the config names, and the
	// token contract on that chain is the one the scanner watches.
	CryptoNetworkHealthOK = "ok"
	// CryptoNetworkHealthMismatch — the configuration is provably wrong. The chain
	// behind the RPC, or the token contract on it, is not what the config claims.
	// USDT sent to such a network can never be credited to anyone, so the network
	// stops being offered for payment until the configuration is fixed.
	//
	// This is not hypothetical. Production shipped a TRON contract address that does
	// not exist on TRON mainnet and a Solana RPC pointed at devnet, and both chains
	// went on quietly accepting orders for months.
	CryptoNetworkHealthMismatch = "mismatch"
	// CryptoNetworkHealthUnknown — nothing could be checked, because no endpoint
	// answered. This deliberately does NOT stop payments: a deposit is matched later
	// from the scanner cursor, so an RPC outage costs time, not money, and must not
	// take the payment page down with it.
	CryptoNetworkHealthUnknown = "unknown"
)

// CryptoNetworkHealth is the verdict of the last preflight check of one network.
type CryptoNetworkHealth struct {
	Network   string    `json:"network"`
	Verdict   string    `json:"verdict"`
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

var (
	cryptoNetworkHealthMutex sync.RWMutex
	cryptoNetworkHealth      = map[string]CryptoNetworkHealth{}
)

func SetCryptoNetworkHealth(health CryptoNetworkHealth) {
	cryptoNetworkHealthMutex.Lock()
	defer cryptoNetworkHealthMutex.Unlock()
	cryptoNetworkHealth[health.Network] = health
}

// GetCryptoNetworkHealthOf returns the last verdict for one network, or the zero
// value when it has never been checked.
func GetCryptoNetworkHealthOf(network string) CryptoNetworkHealth {
	cryptoNetworkHealthMutex.RLock()
	defer cryptoNetworkHealthMutex.RUnlock()
	return cryptoNetworkHealth[network]
}

func GetCryptoNetworkHealth() []CryptoNetworkHealth {
	cryptoNetworkHealthMutex.RLock()
	defer cryptoNetworkHealthMutex.RUnlock()
	report := make([]CryptoNetworkHealth, 0, len(cryptoNetworkHealth))
	for _, health := range cryptoNetworkHealth {
		report = append(report, health)
	}
	sort.Slice(report, func(i int, j int) bool { return report[i].Network < report[j].Network })
	return report
}

// CryptoNetworkIsPayable reports whether a user may still be asked to send money to
// this network. Only a proven mismatch withdraws it; an unchecked or unreachable
// network stays open.
func CryptoNetworkIsPayable(network string) bool {
	cryptoNetworkHealthMutex.RLock()
	defer cryptoNetworkHealthMutex.RUnlock()
	return cryptoNetworkHealth[network].Verdict != CryptoNetworkHealthMismatch
}

// cryptoPublicRPCFallbacks are keyless public mainnet endpoints, appended after
// whatever the operator configured and tried only when those fail.
//
// A receive path must not rest on one provider. Production's Solana endpoint turned
// out to be pointed at devnet, and the API key that owned it had no Solana mainnet
// access at all — so the chain had nowhere to fail over to, and a single provider
// deciding to rate-limit would have been just as final. A wrong-chain fallback
// cannot do damage here: preflight asks every endpoint which chain it serves and
// drops the ones that answer wrong.
// Each of these was checked to answer on the right chain. polygon-rpc.com is
// deliberately absent: it now replies 401 "tenant disabled" to keyless callers,
// which is what a fallback list rots into if nobody ever looks at it.
//
// getLogs capability was re-verified 2026-07-28, and it is the axis that matters:
// an endpoint that serves eth_blockNumber but refuses eth_getLogs keeps the chain
// head looking fresh while every deposit goes unseen. bsc-dataseed (-32005 "limit
// exceeded" even for 10 blocks at head) and publicnode ("Archive requests require
// a personal token" even at head) both refuse getLogs outright now; they stay
// listed last because their head reads are still useful and the refusal
// classification in the scanner rotates past them. blastapi.io was dropped: it
// answers every request with "Blast API is no longer available".
var cryptoPublicRPCFallbacks = map[string][]string{
	// bsc.drpc.org served a 500-block getLogs at head (retains ~6 days of
	// history); rpc-bsc.48.club the same; 1rpc.io/bnb caps getLogs at 50 blocks
	// but reaches arbitrarily old history.
	"bsc_erc20": {
		"https://bsc.drpc.org",
		"https://rpc-bsc.48.club",
		"https://1rpc.io/bnb",
		"https://bsc-dataseed.bnbchain.org",
		"https://bsc-rpc.publicnode.com",
	},
	// polygon.drpc.org refuses keyless getLogs with a lying range complaint;
	// 1rpc.io/matic caps at 50 blocks but serves them.
	"polygon_pos": {
		"https://1rpc.io/matic",
		"https://polygon.drpc.org",
		"https://polygon-bor-rpc.publicnode.com",
	},
	"tron_trc20": {"https://api.trongrid.io"},
	"solana":     {"https://solana-rpc.publicnode.com", "https://api.mainnet-beta.solana.com"},
}

// CryptoRPCEndpoints returns the endpoints for a network, most preferred first: the
// configured ones (the option accepts a whitespace or comma separated list), then
// the public fallbacks.
func CryptoRPCEndpoints(network string) string {
	configured := ""
	switch network {
	case "bsc_erc20":
		configured = CryptoBSCRPCURL
	case "polygon_pos":
		configured = CryptoPolygonRPCURL
	case "tron_trc20":
		configured = CryptoTronRPCURL
	case "solana":
		configured = CryptoSolanaRPCURL
	}
	endpoints := append([]string{}, strings.TrimSpace(configured))
	endpoints = append(endpoints, cryptoPublicRPCFallbacks[network]...)
	return strings.Join(endpoints, " ")
}

// GetPayableCryptoPaymentNetworks returns the networks a user may pay on right now.
//
// It is deliberately narrower than GetEnabledCryptoPaymentNetworks, which the
// scanner keeps using: a network taken off sale must still be scanned, or a deposit
// already in flight when the misconfiguration was caught would never be credited.
func GetPayableCryptoPaymentNetworks() []CryptoPaymentNetworkConfig {
	networks := GetEnabledCryptoPaymentNetworks()
	payable := make([]CryptoPaymentNetworkConfig, 0, len(networks))
	for _, network := range networks {
		if CryptoNetworkIsPayable(network.Network) {
			payable = append(payable, network)
		}
	}
	return payable
}
