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
// seconds. It never grows back; a restart is what re-opens the bidding.
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
