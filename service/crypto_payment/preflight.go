package crypto_payment

import (
	"context"
	"fmt"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

// ChainVerifier proves, against the live chain, that a network's configuration can
// actually collect money.
//
// Every fault this subsystem has shipped was silent. A TRON contract address that
// exists on no chain. An RPC pointed at devnet. A Solana query that structurally
// cannot see incoming SPL transfers. Each one looked exactly like "nobody has paid
// yet", and each one let users go on sending USDT to an address nothing was
// watching. A scanner that finds nothing cannot tell you whether it is configured
// correctly — but the chain can be asked directly, and it answers in seconds.
type ChainVerifier interface {
	Verify(ctx context.Context) setting.CryptoNetworkHealth
}

// preflightInterval re-checks a network that is already running. Configuration is
// edited in the admin UI at runtime, and an endpoint can be repointed at another
// cluster without anything here restarting.
const preflightInterval = 5 * time.Minute

func healthOK(network string) setting.CryptoNetworkHealth {
	return setting.CryptoNetworkHealth{Network: network, Verdict: setting.CryptoNetworkHealthOK, CheckedAt: time.Now()}
}

func healthMismatch(network string, detail string) setting.CryptoNetworkHealth {
	return setting.CryptoNetworkHealth{Network: network, Verdict: setting.CryptoNetworkHealthMismatch, Detail: detail, CheckedAt: time.Now()}
}

func healthUnknown(network string, detail string) setting.CryptoNetworkHealth {
	return setting.CryptoNetworkHealth{Network: network, Verdict: setting.CryptoNetworkHealthUnknown, Detail: detail, CheckedAt: time.Now()}
}

// runPreflight checks one network and publishes the verdict, which is what decides
// whether users are still offered this network to pay on.
func runPreflight(ctx context.Context, scanner NetworkScanner) {
	verifier, ok := scanner.(ChainVerifier)
	if !ok {
		return
	}
	network := scanner.Network()
	previous := setting.CryptoNetworkIsPayable(network)
	health := verifier.Verify(ctx)
	setting.SetCryptoNetworkHealth(health)

	switch health.Verdict {
	case setting.CryptoNetworkHealthMismatch:
		message := fmt.Sprintf("crypto payment network %s is misconfigured and has been withdrawn from checkout: %s", network, health.Detail)
		common.SysLog(message)
		if previous {
			// The admin log is the only channel an operator actually reads. A network
			// that stops being able to take money must not be discoverable only by
			// grepping the container's stdout.
			model.RecordLog(0, model.LogTypeSystem, message)
		}
	case setting.CryptoNetworkHealthUnknown:
		common.SysLog(fmt.Sprintf("crypto payment network %s could not be verified, leaving it open: %s", network, health.Detail))
	default:
		if !previous {
			message := fmt.Sprintf("crypto payment network %s passed verification and is available for checkout again", network)
			common.SysLog(message)
			model.RecordLog(0, model.LogTypeSystem, message)
		}
	}
}

// verifyEVMNetwork answers three questions the scanner cannot: is this endpoint on
// the chain we think it is, is anything actually deployed at the contract address,
// and does that token count in the decimals we bill by.
func verifyEVMNetwork(
	ctx context.Context,
	network string,
	rpc *evmRPCClient,
	expectedChainID int64,
	config setting.CryptoPaymentNetworkConfig,
) setting.CryptoNetworkHealth {
	endpoints := rpc.pool.all()
	if len(endpoints) == 0 {
		return healthUnknown(network, "no RPC endpoint is configured")
	}

	answered := 0
	for _, endpoint := range endpoints {
		chainID, err := rpc.chainIDAt(ctx, endpoint)
		if err != nil {
			// An endpoint that did not answer has proved nothing. Evicting on a timeout
			// would let one bad minute permanently shrink the pool.
			continue
		}
		answered++
		if chainID != expectedChainID {
			rpc.pool.evict(endpoint, fmt.Sprintf("it serves chain id %d, but %s is chain id %d", chainID, network, expectedChainID))
		}
	}
	if answered == 0 {
		return healthUnknown(network, "no RPC endpoint answered")
	}
	if rpc.pool.size() == 0 {
		return healthMismatch(network, fmt.Sprintf(
			"every configured RPC endpoint serves a different chain (expected chain id %d)", expectedChainID))
	}

	exists, err := rpc.contractExists(ctx, config.Contract)
	if err != nil {
		return healthUnknown(network, "could not read the token contract: "+err.Error())
	}
	if !exists {
		return healthMismatch(network, fmt.Sprintf(
			"no contract is deployed at %s, so it can never emit the transfer a deposit is matched by", config.Contract))
	}

	decimals, err := rpc.tokenDecimals(ctx, config.Contract)
	if err != nil {
		// Only a contradiction disqualifies a network. An unanswered question is not one.
		common.SysLog(fmt.Sprintf("crypto payment network %s: could not read token decimals (%s), continuing", network, err.Error()))
		return healthOK(network)
	}
	if decimals != config.Decimals {
		return healthMismatch(network, decimalsMismatchDetail(config.Contract, decimals, config.Decimals))
	}
	return healthOK(network)
}

// decimalsMismatchDetail spells out the consequence, because the number itself does
// not look alarming: USDT is 6 decimals on Polygon and TRON but 18 on BSC, and
// getting it wrong scales every payment by a factor of a trillion in one direction
// or the other.
func decimalsMismatchDetail(contract string, onChain int, configured int) string {
	gap := onChain - configured
	if gap < 0 {
		gap = -gap
	}
	return fmt.Sprintf(
		"token %s reports %d decimals but this network is configured for %d, so every amount would be wrong by a factor of 10^%d",
		contract, onChain, configured, gap)
}
