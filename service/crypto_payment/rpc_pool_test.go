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
