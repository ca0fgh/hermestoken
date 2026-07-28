package crypto_payment

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// ankrPolygonLike mimics the provider that took the Polygon scanner down: it
// serves eth_getLogs only when toBlock-fromBlock <= 100, and rejects anything
// wider with the message it actually returns. Ankr's BSC endpoint — same vendor,
// same API key — serves 500, which is why only Polygon died.
func ankrPolygonLike(scanned *[]int64) func(context.Context, int64, int64) ([]bscRPCLog, error) {
	return func(_ context.Context, fromBlock int64, toBlock int64) ([]bscRPCLog, error) {
		if toBlock-fromBlock > 100 {
			return nil, fmt.Errorf("Polygon RPC error -32062: Block range is too large")
		}
		for block := fromBlock; block <= toBlock; block++ {
			*scanned = append(*scanned, block)
		}
		return nil, nil
	}
}

func noopHandle([]bscRPCLog) error { return nil }

func TestScanEVMRangeNegotiatesSpanDownAndScansWholeRange(t *testing.T) {
	var scanned []int64
	span := int64(evmMaxBlockSpan)

	lastScanned, err := scanEVMRange(context.Background(), &span, 1000, 2000, ankrPolygonLike(&scanned), noopHandle)
	if err != nil {
		t.Fatalf("expected the scan to survive a provider that caps the span, got %v", err)
	}
	if lastScanned != 2000 {
		t.Fatalf("expected the whole range to be scanned, stopped at %d", lastScanned)
	}
	if span > 100 {
		t.Fatalf("expected the span to be negotiated down to what the provider serves, still %d", span)
	}
	// Every block exactly once: no gap a deposit could hide in, no double credit.
	if len(scanned) != 1001 {
		t.Fatalf("expected 1001 blocks scanned exactly once, got %d", len(scanned))
	}
	for i, block := range scanned {
		if block != int64(1000+i) {
			t.Fatalf("expected contiguous blocks, got %d at index %d", block, i)
		}
	}
}

// The stall was not the rejected request; it was losing the cursor when one came
// back. Progress ahead of a failing chunk must survive, or the next tick replays
// the same doomed request forever.
func TestScanEVMRangeReturnsProgressMadeBeforeAFailure(t *testing.T) {
	span := int64(100)
	boom := errors.New("connection reset by peer")
	getLogs := func(_ context.Context, fromBlock int64, _ int64) ([]bscRPCLog, error) {
		if fromBlock >= 1200 {
			return nil, boom
		}
		return nil, nil
	}

	lastScanned, err := scanEVMRange(context.Background(), &span, 1000, 5000, getLogs, noopHandle)
	if !errors.Is(err, boom) {
		t.Fatalf("expected the failure to surface, got %v", err)
	}
	if lastScanned != 1199 {
		t.Fatalf("expected the cursor to hold the blocks already scanned, got %d", lastScanned)
	}
}

// A scan that only ever advanced one chunk per tick could never close a 4M-block
// backlog: Polygon produces blocks faster than 100-per-10s consumes them.
func TestScanEVMRangeWalksManyChunksPerCall(t *testing.T) {
	span := int64(100)
	calls := 0
	getLogs := func(_ context.Context, _ int64, _ int64) ([]bscRPCLog, error) {
		calls++
		return nil, nil
	}

	lastScanned, err := scanEVMRange(context.Background(), &span, 0, 999, getLogs, noopHandle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 10 {
		t.Fatalf("expected the backlog to be walked in many chunks, made only %d call(s)", calls)
	}
	if lastScanned != 999 {
		t.Fatalf("expected the backlog to be consumed, stopped at %d", lastScanned)
	}
}

// Shrinking has a floor, so a provider that rejects even the smallest span must
// surface an error rather than spin.
func TestScanEVMRangeGivesUpWhenEvenTheMinimumSpanIsRejected(t *testing.T) {
	span := int64(evmMaxBlockSpan)
	getLogs := func(_ context.Context, _ int64, _ int64) ([]bscRPCLog, error) {
		return nil, errors.New("block range is too large")
	}

	lastScanned, err := scanEVMRange(context.Background(), &span, 1000, 5000, getLogs, noopHandle)
	if err == nil {
		t.Fatal("expected an error when the provider rejects even the minimum span")
	}
	if lastScanned != 999 {
		t.Fatalf("expected no progress to be claimed, got %d", lastScanned)
	}
	if span != evmMinBlockSpan {
		t.Fatalf("expected the span to bottom out at the floor, got %d", span)
	}
}

func TestIsBlockRangeTooLarge(t *testing.T) {
	tooLarge := []string{
		// Verbatim from the production log that stalled Polygon for two months.
		"Polygon RPC error -32062: Block range is too large",
		"eth_getLogs is limited to 0 - 50 blocks",
		"ranges over 10000 blocks are not supported on freetier",
		"query returned more than 10000 results",
		// Verbatim from blastapi and 1rpc, 2026-07-28. Neither matched the older
		// hints, and an unmatched range cap is a permanently stuck scanner.
		"You can make eth_getLogs requests with up to a 10 block range. Based on your parameters, this block range should work: [0x6a49262, 0x6a4926b]",
		"BSC RPC error -32602: eth_getLogs is limited to 0 - 50 blocks range",
	}
	for _, message := range tooLarge {
		if !isBlockRangeTooLarge(errors.New(message)) {
			t.Errorf("expected %q to be treated as a span the provider will not serve", message)
		}
	}

	// Shrinking the span cannot fix these, and retrying a smaller one would just
	// hammer the provider.
	other := []string{
		"connection reset by peer",
		"Polygon RPC HTTP status 429",
		"invalid api key",
	}
	for _, message := range other {
		if isBlockRangeTooLarge(errors.New(message)) {
			t.Errorf("expected %q not to be mistaken for an oversized span", message)
		}
	}
	if isBlockRangeTooLarge(nil) {
		t.Error("expected nil not to be an oversized span")
	}
}

func TestIsProviderRefusal(t *testing.T) {
	// Each phrase verbatim from a live provider, 2026-07-28. Every one of these is
	// the provider declining to serve, which only another provider can fix.
	refusals := []string{
		"limit exceeded", // bsc-dataseed -32005, its answer to every getLogs
		"Archive requests require a personal token. Get one at: https://www.allnodes.com/publicnode",
		"You reached Public endpoint rate limit, please upgrade to paid plan",
		"The method eth_getLogs is not supported.",
		"no available upstreams to process a request",
		"header not found",
	}
	for _, message := range refusals {
		if !isProviderRefusal(message) {
			t.Errorf("expected %q to be recognized as the provider refusing to serve", message)
		}
	}

	// Real answers and range negotiation must not be mistaken for refusal.
	notRefusals := []string{
		"execution reverted",
		"invalid argument 0: json: cannot unmarshal",
	}
	for _, message := range notRefusals {
		if isProviderRefusal(message) {
			t.Errorf("expected %q not to be treated as a refusal", message)
		}
	}
}
