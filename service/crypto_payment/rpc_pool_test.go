package crypto_payment

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitEndpointsAcceptsAListInTheSingleOptionField(t *testing.T) {
	endpoints := splitEndpoints(" https://a.example/rpc/ ,https://b.example\nhttps://a.example/rpc ")

	// Trailing slashes are normalized away, so the same endpoint written two ways is
	// not two endpoints.
	assert.Equal(t, []string{"https://a.example/rpc", "https://b.example"}, endpoints)
}

// A provider that rate-limits or 403s must not take the chain down with it.
func TestEndpointPoolFailsOverWhenAnEndpointIsUnavailable(t *testing.T) {
	pool := newEndpointPool("solana", "https://down.example https://up.example")
	tried := []string{}

	err := pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		if endpoint == "https://down.example" {
			return endpointUnavailable(fmt.Errorf("HTTP status 429"))
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"https://down.example", "https://up.example"}, tried)

	// The endpoint that answered is now current, so a healthy provider is not
	// rediscovered on every single call.
	tried = tried[:0]
	require.NoError(t, pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		return nil
	}))
	assert.Equal(t, []string{"https://up.example"}, tried)
}

// A JSON-RPC error is the provider answering us, not failing. Retrying it elsewhere
// would hide the "block range too large" negotiation that scanEVMRange depends on —
// and that negotiation is the thing that unstuck Polygon.
func TestEndpointPoolDoesNotFailOverOnAnAnswerItDoesNotLike(t *testing.T) {
	pool := newEndpointPool("polygon_pos", "https://a.example https://b.example")
	tried := []string{}

	err := pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		return fmt.Errorf("block range is too large")
	})

	require.Error(t, err)
	assert.Equal(t, []string{"https://a.example"}, tried)
	assert.True(t, isBlockRangeTooLarge(err), "the caller must still see the range error verbatim")
}

func TestEndpointPoolReportsWhenEveryEndpointIsGone(t *testing.T) {
	pool := newEndpointPool("solana", "https://only.example")
	pool.evict("https://only.example", "it is on devnet")

	err := pool.do(func(endpoint string) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no usable RPC endpoint")
}

// Provider URLs carry API keys in the path. A log line that names the provider is
// useful; one that leaks the key is a liability.
func TestRedactEndpointKeepsTheHostAndDropsTheKey(t *testing.T) {
	assert.Equal(t, "https://rpc.ankr.com/***", redactEndpoint("https://rpc.ankr.com/solana/2f9a...secret"))
	assert.Equal(t, "https://solana-rpc.publicnode.com", redactEndpoint("https://solana-rpc.publicnode.com"))
}

// One transient blip on the paid primary must not demote the pool onto a free
// fallback forever. After reauditionInterval the primary gets asked again.
func TestEndpointPoolReauditionsThePrimaryAfterAWhile(t *testing.T) {
	pool := newEndpointPool("bsc_erc20", "https://primary.example https://fallback.example")
	pool.promote("https://fallback.example")

	tried := []string{}
	require.NoError(t, pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		return nil
	}))
	require.Equal(t, []string{"https://fallback.example"}, tried, "the fallback holds current before the interval passes")

	pool.mu.Lock()
	pool.demotedAt = pool.demotedAt.Add(-reauditionInterval)
	pool.mu.Unlock()

	tried = tried[:0]
	require.NoError(t, pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		return nil
	}))
	assert.Equal(t, []string{"https://primary.example"}, tried)
}

// The stall reset must start the next call from the operator's first choice, no
// matter which endpoint talked its way into being current.
func TestResetToPrimaryStartsOverFromTheConfiguredEndpoint(t *testing.T) {
	pool := newEndpointPool("bsc_erc20", "https://primary.example https://fallback.example")
	pool.promote("https://fallback.example")

	pool.resetToPrimary()

	tried := []string{}
	require.NoError(t, pool.do(func(endpoint string) error {
		tried = append(tried, endpoint)
		return nil
	}))
	assert.Equal(t, []string{"https://primary.example"}, tried)
}

// Go's HTTP client quotes the full request URL in its errors, and provider URLs
// carry API keys in the path. Production leaked the Ankr key into the system log
// this way: the endpoint label was redacted, the error text was not.
func TestFailoverLogAndSweepErrorDoNotLeakTheAPIKey(t *testing.T) {
	keyed := "https://rpc.ankr.com/bsc/deadbeefsecret"
	pool := newEndpointPool("bsc_erc20", keyed+" https://fallback.example")

	err := pool.do(func(endpoint string) error {
		return endpointUnavailable(fmt.Errorf("Post %q: connection reset by peer", endpoint))
	})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "deadbeefsecret")
	assert.Contains(t, err.Error(), "https://fallback.example")
}
