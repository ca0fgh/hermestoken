package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const bscTransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

type BSCScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewBSCScanner(config setting.CryptoPaymentNetworkConfig) *BSCScanner {
	return &BSCScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *BSCScanner) Network() string { return model.CryptoNetworkBSCERC20 }

func (s *BSCScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoBSCRPCURL) == "" {
		return fmt.Errorf("BSC RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromBlock := currentBlock - int64(s.config.Confirmations) - 30
	if err == nil && state.LastScannedBlock > 0 {
		fromBlock = state.LastScannedBlock + 1
	}
	if fromBlock < 0 {
		fromBlock = 0
	}
	toBlock := fromBlock + 500
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if toBlock > maxSafe {
		toBlock = maxSafe
	}
	if toBlock < fromBlock {
		return nil
	}
	logs, err := s.getLogs(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}
	blockTimestamps := make(map[int64]int64)
	for _, item := range logs {
		transfer, err := decodeBSCTransferLog(item, s.config.Decimals)
		if err != nil {
			return err
		}
		if !strings.EqualFold(transfer.ToAddress, s.config.ReceiveAddress) {
			continue
		}
		blockTimestamp, ok := blockTimestamps[transfer.BlockNumber]
		if !ok {
			blockTimestamp, err = s.blockTimestamp(ctx, transfer.BlockNumber)
			if err != nil {
				return err
			}
			blockTimestamps[transfer.BlockNumber] = blockTimestamp
		}
		transfer.Network = s.Network()
		transfer.TokenContract = s.config.Contract
		transfer.BlockTimestamp = blockTimestamp
		transfer.Confirmations = currentBlock - transfer.BlockNumber + 1
		transfer.ObservedAt = time.Now()
		if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
			return err
		}
	}
	return model.UpsertCryptoScannerState(s.Network(), toBlock, maxSafe)
}

type bscRPCLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	LogIndex    string   `json:"logIndex"`
}

func decodeBSCTransferLog(log bscRPCLog, decimals int) (model.CryptoObservedTransfer, error) {
	if len(log.Topics) < 3 || strings.ToLower(log.Topics[0]) != bscTransferTopic {
		return model.CryptoObservedTransfer{}, fmt.Errorf("not a transfer log")
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(strings.TrimPrefix(log.Data, "0x"), 16); !ok {
		return model.CryptoObservedTransfer{}, fmt.Errorf("invalid transfer amount")
	}
	blockNumber, err := parseHexInt64(log.BlockNumber)
	if err != nil {
		return model.CryptoObservedTransfer{}, err
	}
	logIndex, err := parseHexInt64(log.LogIndex)
	if err != nil {
		return model.CryptoObservedTransfer{}, err
	}
	return model.CryptoObservedTransfer{
		TxHash:          log.TxHash,
		LogIndex:        int(logIndex),
		BlockNumber:     blockNumber,
		FromAddress:     topicToEVMAddress(log.Topics[1]),
		ToAddress:       topicToEVMAddress(log.Topics[2]),
		TokenContract:   log.Address,
		TokenDecimals:   decimals,
		AmountBaseUnits: amount.String(),
	}, nil
}

func topicToEVMAddress(topic string) string {
	trimmed := strings.TrimPrefix(topic, "0x")
	if len(trimmed) < 40 {
		return "0x" + strings.ToLower(trimmed)
	}
	return "0x" + strings.ToLower(trimmed[len(trimmed)-40:])
}

func parseHexInt64(value string) (int64, error) {
	trimmed := strings.TrimPrefix(value, "0x")
	parsed, err := strconv.ParseInt(trimmed, 16, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

type bscRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type bscRPCResponse struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type evmRPCBlock struct {
	Timestamp string `json:"timestamp"`
}

func (s *BSCScanner) currentBlock(ctx context.Context) (int64, error) {
	var result string
	if err := s.rpc(ctx, "eth_blockNumber", nil, &result); err != nil {
		return 0, err
	}
	return parseHexInt64(result)
}

func (s *BSCScanner) blockTimestamp(ctx context.Context, blockNumber int64) (int64, error) {
	var block evmRPCBlock
	if err := s.rpc(ctx, "eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blockNumber), false}, &block); err != nil {
		return 0, err
	}
	if strings.TrimSpace(block.Timestamp) == "" {
		return 0, fmt.Errorf("BSC block %d response missing timestamp", blockNumber)
	}
	return parseHexInt64(block.Timestamp)
}

func (s *BSCScanner) getLogs(ctx context.Context, fromBlock int64, toBlock int64) ([]bscRPCLog, error) {
	filter := map[string]interface{}{
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"address":   s.config.Contract,
		"topics": []interface{}{
			bscTransferTopic,
			nil,
			"0x000000000000000000000000" + strings.TrimPrefix(strings.ToLower(s.config.ReceiveAddress), "0x"),
		},
	}
	var logs []bscRPCLog
	if err := s.rpc(ctx, "eth_getLogs", []interface{}{filter}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *BSCScanner) rpc(ctx context.Context, method string, params []interface{}, out interface{}) error {
	if params == nil {
		params = []interface{}{}
	}
	payload, err := common.Marshal(bscRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.CryptoBSCRPCURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("BSC RPC HTTP status %d", resp.StatusCode)
	}
	var envelope bscRPCResponse
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("BSC RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	encoded, err := common.Marshal(envelope.Result)
	if err != nil {
		return err
	}
	return common.Unmarshal(encoded, out)
}
