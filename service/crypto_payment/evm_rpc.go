package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

// erc20DecimalsSelector is the first four bytes of keccak256("decimals()").
const erc20DecimalsSelector = "0x313ce567"

type evmRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type evmRPCResponse struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type evmRPCBlock struct {
	Timestamp string `json:"timestamp"`
}

// evmRPCClient is the JSON-RPC transport shared by every EVM chain we scan.
type evmRPCClient struct {
	network string
	client  *http.Client
	pool    *endpointPool
}

func newEVMRPCClient(network string, endpoints string) *evmRPCClient {
	return &evmRPCClient{
		network: network,
		client:  &http.Client{Timeout: 15 * time.Second},
		pool:    newEndpointPool(network, endpoints),
	}
}

// call sends method to whichever endpoint is currently healthy, failing over to the
// others when the endpoint itself is the problem.
func (c *evmRPCClient) call(ctx context.Context, method string, params []interface{}, out interface{}) error {
	return c.pool.do(func(endpoint string) error {
		return c.callAt(ctx, endpoint, method, params, out)
	})
}

// callAt pins the request to one endpoint. Preflight needs this: "which chain is
// this endpoint serving" is a question about a specific provider, and failing over
// mid-question would answer it about a different one.
func (c *evmRPCClient) callAt(ctx context.Context, endpoint string, method string, params []interface{}, out interface{}) error {
	if params == nil {
		params = []interface{}{}
	}
	payload, err := common.Marshal(evmRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return endpointUnavailable(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return endpointUnavailable(fmt.Errorf("%s RPC HTTP status %d", c.network, resp.StatusCode))
	}
	var envelope evmRPCResponse
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		err := fmt.Errorf("%s RPC error %d: %s", c.network, envelope.Error.Code, envelope.Error.Message)
		// Range feedback must stay on this endpoint: scanEVMRange answers it by
		// shrinking the span, and failing over would hide the negotiation. Checked
		// first because providers mix vocabularies — dRPC phrases its keyless
		// getLogs refusal as a range complaint ("ranges over 10000 blocks are not
		// supported on free plan") even for a 500-block ask.
		if isBlockRangeTooLarge(err) {
			return err
		}
		if isProviderRefusal(envelope.Error.Message) {
			return endpointUnavailable(err)
		}
		return err
	}
	encoded, err := common.Marshal(envelope.Result)
	if err != nil {
		return err
	}
	return common.Unmarshal(encoded, out)
}

func (c *evmRPCClient) currentBlock(ctx context.Context) (int64, error) {
	var result string
	if err := c.call(ctx, "eth_blockNumber", nil, &result); err != nil {
		return 0, err
	}
	return parseHexInt64(result)
}

func (c *evmRPCClient) blockTimestamp(ctx context.Context, blockNumber int64) (int64, error) {
	var block evmRPCBlock
	if err := c.call(ctx, "eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blockNumber), false}, &block); err != nil {
		return 0, err
	}
	if strings.TrimSpace(block.Timestamp) == "" {
		return 0, fmt.Errorf("%s block %d response missing timestamp", c.network, blockNumber)
	}
	return parseHexInt64(block.Timestamp)
}

func (c *evmRPCClient) transferLogs(ctx context.Context, contract string, receiveAddress string, fromBlock int64, toBlock int64) ([]bscRPCLog, error) {
	filter := map[string]interface{}{
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"address":   contract,
		"topics": []interface{}{
			bscTransferTopic,
			nil,
			"0x000000000000000000000000" + strings.TrimPrefix(strings.ToLower(receiveAddress), "0x"),
		},
	}
	var logs []bscRPCLog
	if err := c.call(ctx, "eth_getLogs", []interface{}{filter}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (c *evmRPCClient) chainIDAt(ctx context.Context, endpoint string) (int64, error) {
	var result string
	if err := c.callAt(ctx, endpoint, "eth_chainId", nil, &result); err != nil {
		return 0, err
	}
	return parseHexInt64(result)
}

// contractExists reports whether anything is deployed at the address. An address
// with no code emits no events, which is what a scanner watching it sees: silence,
// indistinguishable from "nobody has paid yet".
func (c *evmRPCClient) contractExists(ctx context.Context, contract string) (bool, error) {
	var code string
	if err := c.call(ctx, "eth_getCode", []interface{}{contract, "latest"}, &code); err != nil {
		return false, err
	}
	return len(strings.TrimPrefix(strings.TrimSpace(code), "0x")) > 0, nil
}

func (c *evmRPCClient) tokenDecimals(ctx context.Context, contract string) (int, error) {
	var result string
	err := c.call(ctx, "eth_call", []interface{}{
		map[string]interface{}{"to": contract, "data": erc20DecimalsSelector},
		"latest",
	}, &result)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimPrefix(strings.TrimSpace(result), "0x")
	if trimmed == "" {
		return 0, fmt.Errorf("%s contract %s returned no decimals", c.network, contract)
	}
	decimals, ok := new(big.Int).SetString(trimmed, 16)
	if !ok || !decimals.IsInt64() {
		return 0, fmt.Errorf("%s contract %s returned an unreadable decimals value %q", c.network, contract, result)
	}
	return int(decimals.Int64()), nil
}
