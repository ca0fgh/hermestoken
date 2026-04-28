package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

type PolygonScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewPolygonScanner(config setting.CryptoPaymentNetworkConfig) *PolygonScanner {
	return &PolygonScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *PolygonScanner) Network() string { return model.CryptoNetworkPolygonPOS }

func (s *PolygonScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoPolygonRPCURL) == "" {
		return fmt.Errorf("Polygon RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromBlock := currentBlock - int64(s.config.Confirmations) - 60
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

func (s *PolygonScanner) currentBlock(ctx context.Context) (int64, error) {
	var result string
	if err := s.rpc(ctx, "eth_blockNumber", nil, &result); err != nil {
		return 0, err
	}
	return parseHexInt64(result)
}

func (s *PolygonScanner) blockTimestamp(ctx context.Context, blockNumber int64) (int64, error) {
	var block evmRPCBlock
	if err := s.rpc(ctx, "eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blockNumber), false}, &block); err != nil {
		return 0, err
	}
	if strings.TrimSpace(block.Timestamp) == "" {
		return 0, fmt.Errorf("Polygon block %d response missing timestamp", blockNumber)
	}
	return parseHexInt64(block.Timestamp)
}

func (s *PolygonScanner) getLogs(ctx context.Context, fromBlock int64, toBlock int64) ([]bscRPCLog, error) {
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

func (s *PolygonScanner) rpc(ctx context.Context, method string, params []interface{}, out interface{}) error {
	if params == nil {
		params = []interface{}{}
	}
	payload, err := common.Marshal(bscRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.CryptoPolygonRPCURL, bytes.NewReader(payload))
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
		return fmt.Errorf("Polygon RPC HTTP status %d", resp.StatusCode)
	}
	var envelope bscRPCResponse
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("Polygon RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	encoded, err := common.Marshal(envelope.Result)
	if err != nil {
		return err
	}
	return common.Unmarshal(encoded, out)
}
