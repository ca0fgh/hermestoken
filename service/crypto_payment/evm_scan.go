package crypto_payment

import (
	"context"
	"strings"
	"time"
)

const (
	// Starting span for eth_getLogs. Providers disagree wildly on what they will
	// serve (Ankr's Polygon endpoint caps at toBlock-fromBlock <= 100, others
	// happily serve thousands), so this is only an opening bid: scanEVMRange
	// negotiates it down against whatever the provider actually accepts.
	evmMaxBlockSpan = 500
	evmMinBlockSpan = 10
	// A scan resumes from the persisted cursor on the next tick, so it does not
	// need to finish the backlog in one pass. Bounding it keeps a catching-up
	// scanner from outliving the 30s Redis lock it holds.
	evmScanTimeBudget = 8 * time.Second
)

// Providers phrase "you asked for too many blocks at once" every which way, and
// some report it as a result-count cap rather than a range cap. The remedy is
// identical in every case, so they are all treated as one condition.
var blockRangeTooLargeHints = []string{
	"block range is too large",
	"block range too large",
	"exceed maximum block range",
	"exceeds max block range",
	"range is too large",
	"ranges over",
	"query returned more than",
	"log response size exceeded",
	"too many blocks",
	"is limited to",
	// blastapi: "You can make eth_getLogs requests with up to a 10 block range".
	"block range",
	// 1rpc: "eth_getLogs is limited to 0 - 50 blocks range".
	"blocks range",
}

func isBlockRangeTooLarge(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, hint := range blockRangeTooLargeHints {
		if strings.Contains(message, hint) {
			return true
		}
	}
	return false
}

// A provider refusing to serve — rate limits, quotas, paywalled methods, pruned
// history — is an endpoint problem, not feedback about the request, and the only
// correct response is to ask a different provider. Treating these as ordinary
// JSON-RPC answers is what blinded the BSC scanner for five days: bsc-dataseed
// answers every eth_getLogs with -32005 "limit exceeded" (verified even for a
// 10-block range at head), so the pool kept re-asking it forever. Every phrase
// here was seen live from a real provider.
var providerRefusalHints = []string{
	"limit exceeded", // bsc-dataseed's blanket getLogs refusal
	"rate limit",     // 1rpc: "You reached Public endpoint rate limit"
	"too many requests",
	"capacity", // Ankr: compute units per second capacity
	"quota",
	"personal token", // publicnode: getLogs is paywalled
	"api key",
	"upgrade to", // "please upgrade to paid plan"
	"unauthorized",
	"access denied",
	"forbidden",
	"payment required",
	"not supported",          // meowrpc: "The method eth_getLogs is not supported."
	"no available upstreams", // dRPC with no free upstream at the asked height
	"header not found",       // a pruned node asked below its retained window
	"archive",                // "Archive requests require a personal token"
}

func isProviderRefusal(message string) bool {
	lowered := strings.ToLower(message)
	for _, hint := range providerRefusalHints {
		if strings.Contains(lowered, hint) {
			return true
		}
	}
	return false
}

// scanEVMRange walks [fromBlock, maxSafe] in chunks, handing each chunk's logs to
// handle, and returns the highest block it fully scanned.
//
// It returns that cursor even when it also returns an error, and the caller must
// persist it. Not doing so is what took the Polygon scanner down for two months:
// a single rejected eth_getLogs aborted ScanOnce before the cursor was written,
// so the next tick re-issued the same doomed request, forever.
//
// span is owned by the caller (it lives on the scanner) so a span negotiated down
// against the provider survives across ticks instead of being re-learned every 10
// seconds. It never grows back on its own; a restart re-opens the bidding, and so
// does the stall reset in reportScannerProgress, because a span negotiated against
// one endpoint is meaningless against the endpoint the pool fails over to next.
func scanEVMRange(
	ctx context.Context,
	span *int64,
	fromBlock int64,
	maxSafe int64,
	getLogs func(ctx context.Context, fromBlock int64, toBlock int64) ([]bscRPCLog, error),
	handle func(logs []bscRPCLog) error,
) (int64, error) {
	if *span < evmMinBlockSpan {
		*span = evmMaxBlockSpan
	}
	lastScanned := fromBlock - 1
	deadline := time.Now().Add(evmScanTimeBudget)

	for fromBlock <= maxSafe {
		toBlock := fromBlock + *span - 1
		if toBlock > maxSafe {
			toBlock = maxSafe
		}
		logs, err := getLogs(ctx, fromBlock, toBlock)
		if err != nil {
			if isBlockRangeTooLarge(err) && *span > evmMinBlockSpan {
				*span = *span / 2
				if *span < evmMinBlockSpan {
					*span = evmMinBlockSpan
				}
				continue
			}
			return lastScanned, err
		}
		if err := handle(logs); err != nil {
			return lastScanned, err
		}
		lastScanned = toBlock
		fromBlock = toBlock + 1
		if time.Now().After(deadline) {
			break
		}
	}
	return lastScanned, nil
}
