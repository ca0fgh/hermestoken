package crypto_payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonRPCStub(t *testing.T, body string, hits *int) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			*hits++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// bsc-dataseed answers every eth_getLogs with -32005 "limit exceeded" — a
// well-formed JSON-RPC error over HTTP 200. For five days the pool treated that
// as "the provider answered" and kept re-asking it while deposits landed unseen.
// A refusal has to be worth failing over.
func TestCallFailsOverWhenAProviderRefusesToServe(t *testing.T) {
	refusingHits, servingHits := 0, 0
	refusing := jsonRPCStub(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"limit exceeded"}}`, &refusingHits)
	serving := jsonRPCStub(t, `{"jsonrpc":"2.0","id":1,"result":"0x10"}`, &servingHits)

	client := newEVMRPCClient("bsc_erc20", refusing.URL+" "+serving.URL)
	var result string
	err := client.call(context.Background(), "eth_blockNumber", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "0x10", result)
	assert.Equal(t, 1, refusingHits)
	assert.Equal(t, 1, servingHits)
}

// Range feedback is the one JSON-RPC error that must NOT fail over: scanEVMRange
// answers it by shrinking the span, and asking a different provider instead would
// hide the negotiation that unstuck Polygon.
func TestCallDoesNotFailOverOnRangeNegotiationFeedback(t *testing.T) {
	otherHits := 0
	capped := jsonRPCStub(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"eth_getLogs is limited to 0 - 50 blocks range"}}`, nil)
	other := jsonRPCStub(t, `{"jsonrpc":"2.0","id":1,"result":[]}`, &otherHits)

	client := newEVMRPCClient("bsc_erc20", capped.URL+" "+other.URL)
	var result []bscRPCLog
	err := client.call(context.Background(), "eth_getLogs", []interface{}{map[string]interface{}{}}, &result)

	require.Error(t, err)
	assert.True(t, isBlockRangeTooLarge(err), "the caller must see the range error to negotiate on it")
	assert.Equal(t, 0, otherHits, "range feedback must stay on the endpoint that produced it")
}
