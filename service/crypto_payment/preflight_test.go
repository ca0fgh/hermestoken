package crypto_payment

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEVMNode answers the three questions preflight asks. chainID and decimals are
// what a misconfigured deployment gets wrong.
type fakeEVMNode struct {
	chainID  int64
	hasCode  bool
	decimals int64
	calls    map[string]int
}

func newFakeEVMNode(t *testing.T, node *fakeEVMNode) *httptest.Server {
	t.Helper()
	node.calls = map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		request := string(body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request, "eth_chainId"):
			node.calls["eth_chainId"]++
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":"0x%x"}`, node.chainID)
		case strings.Contains(request, "eth_getCode"):
			node.calls["eth_getCode"]++
			if node.hasCode {
				fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x60806040"}`)
				return
			}
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x"}`)
		case strings.Contains(request, "eth_call"):
			node.calls["eth_call"]++
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":"0x%064x"}`, node.decimals)
		default:
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":null}`)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func bscTestConfig(decimals int) setting.CryptoPaymentNetworkConfig {
	return setting.CryptoPaymentNetworkConfig{
		Network:        "bsc_erc20",
		Contract:       "0x55d398326f99059fF775485246999027B3197955",
		ReceiveAddress: "0xaeaf7ECc0337c06911bc01C227135e3646B77580",
		Decimals:       decimals,
	}
}

func TestPreflightAcceptsAChainThatMatchesItsConfiguration(t *testing.T) {
	server := newFakeEVMNode(t, &fakeEVMNode{chainID: bscChainID, hasCode: true, decimals: 18})
	rpc := newEVMRPCClient("bsc_erc20", server.URL)

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	assert.Equal(t, setting.CryptoNetworkHealthOK, health.Verdict)
	assert.Equal(t, 1, rpc.pool.size())
	// A passing verdict has to say what it proved, because runPreflight logs it. A
	// silent pass would leave "all four chains verified" and "the check never ran"
	// looking identical in the logs, which is the exact failure this file exists to
	// end — and I shipped it that way once before catching it in production.
	assert.Contains(t, health.Detail, "chain id 56")
	assert.Contains(t, health.Detail, bscTestConfig(18).Contract)
	assert.Contains(t, health.Detail, "18 decimals")
}

// The TRON contract shipped to production was deployed on no chain at all. A
// contract with no code emits no events, so the scanner's only symptom was finding
// nothing — indistinguishable from nobody having paid yet.
func TestPreflightWithdrawsANetworkWhoseTokenContractDoesNotExist(t *testing.T) {
	server := newFakeEVMNode(t, &fakeEVMNode{chainID: bscChainID, hasCode: false, decimals: 18})
	rpc := newEVMRPCClient("bsc_erc20", server.URL)

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	require.Equal(t, setting.CryptoNetworkHealthMismatch, health.Verdict)
	assert.Contains(t, health.Detail, "no contract is deployed")
}

// USDT is 6 decimals on Polygon and TRON but 18 on BSC. Believing the wrong one
// scales every payment by a factor of a trillion.
func TestPreflightWithdrawsANetworkWhoseTokenDecimalsDisagree(t *testing.T) {
	server := newFakeEVMNode(t, &fakeEVMNode{chainID: bscChainID, hasCode: true, decimals: 6})
	rpc := newEVMRPCClient("bsc_erc20", server.URL)

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	require.Equal(t, setting.CryptoNetworkHealthMismatch, health.Verdict)
	assert.Contains(t, health.Detail, "10^12")
}

// An endpoint on the wrong chain is dropped rather than trusted, and a healthy
// sibling keeps the network open. This is the devnet-RPC failure, healing itself.
func TestPreflightEvictsAnEndpointServingAnotherChainAndKeepsTheGoodOne(t *testing.T) {
	wrongChain := newFakeEVMNode(t, &fakeEVMNode{chainID: 97, hasCode: true, decimals: 18})
	rightChain := newFakeEVMNode(t, &fakeEVMNode{chainID: bscChainID, hasCode: true, decimals: 18})
	rpc := newEVMRPCClient("bsc_erc20", wrongChain.URL+" "+rightChain.URL)
	require.Equal(t, 2, rpc.pool.size())

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	assert.Equal(t, setting.CryptoNetworkHealthOK, health.Verdict)
	assert.Equal(t, []string{rightChain.URL}, rpc.pool.all())
}

// When every endpoint is on the wrong chain there is nothing left to scan with, and
// the network must stop being offered rather than keep collecting money nobody can
// see.
func TestPreflightWithdrawsANetworkWhenEveryEndpointIsOnAnotherChain(t *testing.T) {
	wrongChain := newFakeEVMNode(t, &fakeEVMNode{chainID: 97, hasCode: true, decimals: 18})
	rpc := newEVMRPCClient("bsc_erc20", wrongChain.URL)

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	require.Equal(t, setting.CryptoNetworkHealthMismatch, health.Verdict)
	assert.Contains(t, health.Detail, "different chain")
	assert.Equal(t, 0, rpc.pool.size())
}

// An unreachable endpoint has proved nothing. Withdrawing a network because a
// provider had a bad minute would take payments down for an outage that costs only
// time: a deposit made during one is still matched later, from the cursor.
func TestPreflightLeavesANetworkOpenWhenNoEndpointAnswers(t *testing.T) {
	unreachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
	}))
	t.Cleanup(unreachable.Close)
	rpc := newEVMRPCClient("bsc_erc20", unreachable.URL)

	health := verifyEVMNetwork(context.Background(), "bsc_erc20", rpc, bscChainID, bscTestConfig(18))

	assert.Equal(t, setting.CryptoNetworkHealthUnknown, health.Verdict)
	assert.Equal(t, 1, rpc.pool.size(), "a timeout must not permanently shrink the pool")
	assert.True(t, setting.CryptoNetworkIsPayable("bsc_erc20"))
}
