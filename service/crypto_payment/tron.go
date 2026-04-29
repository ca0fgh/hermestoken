package crypto_payment

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

type TronScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewTronScanner(config setting.CryptoPaymentNetworkConfig) *TronScanner {
	return &TronScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *TronScanner) Network() string { return model.CryptoNetworkTronTRC20 }

func (s *TronScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoTronRPCURL) == "" {
		return fmt.Errorf("TRON RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromBlock := currentBlock - int64(s.config.Confirmations) - 40
	if err == nil && state.LastScannedBlock > 0 {
		fromBlock = state.LastScannedBlock + 1
	}
	if fromBlock < 0 {
		fromBlock = 0
	}
	toBlock := fromBlock + 200
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if toBlock > maxSafe {
		toBlock = maxSafe
	}
	if toBlock < fromBlock {
		return nil
	}
	events, err := s.getTransferEvents(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}
	for _, event := range events {
		transfer, err := decodeTronTransferEvent(event, s.config.Decimals)
		if err != nil {
			return err
		}
		if !strings.EqualFold(transfer.ToAddress, s.config.ReceiveAddress) {
			continue
		}
		transfer.Network = s.Network()
		transfer.TokenContract = s.config.Contract
		transfer.Confirmations = currentBlock - transfer.BlockNumber + 1
		transfer.ObservedAt = time.Now()
		if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
			return err
		}
	}
	return model.UpsertCryptoScannerState(s.Network(), toBlock, maxSafe)
}

type tronGridEventResponse struct {
	Data []tronGridEvent `json:"data"`
}

type tronGridEvent struct {
	TransactionID  string            `json:"transaction_id"`
	BlockNumber    int64             `json:"block_number"`
	BlockTimestamp int64             `json:"block_timestamp"`
	EventIndex     int               `json:"event_index"`
	Result         map[string]string `json:"result"`
}

func decodeTronTransferEvent(event tronGridEvent, decimals int) (model.CryptoObservedTransfer, error) {
	value := strings.TrimSpace(event.Result["value"])
	if value == "" {
		return model.CryptoObservedTransfer{}, fmt.Errorf("missing TRON transfer value")
	}
	return model.CryptoObservedTransfer{
		TxHash:          event.TransactionID,
		LogIndex:        event.EventIndex,
		BlockNumber:     event.BlockNumber,
		BlockTimestamp:  event.BlockTimestamp / 1000,
		FromAddress:     strings.TrimSpace(event.Result["from"]),
		ToAddress:       strings.TrimSpace(event.Result["to"]),
		TokenDecimals:   decimals,
		AmountBaseUnits: value,
	}, nil
}

func (s *TronScanner) currentBlock(ctx context.Context) (int64, error) {
	endpoint := strings.TrimRight(setting.CryptoTronRPCURL, "/") + "/wallet/getnowblock"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if setting.CryptoTronAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", setting.CryptoTronAPIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("TRON RPC HTTP status %d", resp.StatusCode)
	}
	var payload struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return 0, err
	}
	if payload.BlockHeader.RawData.Number <= 0 {
		return 0, fmt.Errorf("TRON current block response missing block number")
	}
	return payload.BlockHeader.RawData.Number, nil
}

func (s *TronScanner) getTransferEvents(ctx context.Context, fromBlock int64, toBlock int64) ([]tronGridEvent, error) {
	base := strings.TrimRight(setting.CryptoTronRPCURL, "/")
	if !strings.Contains(base, "/v1/") {
		base = "https://api.trongrid.io"
	}
	endpoint, err := url.Parse(base + "/v1/contracts/" + url.PathEscape(s.config.Contract) + "/events")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("event_name", "Transfer")
	query.Set("only_confirmed", "false")
	query.Set("limit", "200")
	query.Set("order_by", "block_timestamp,asc")
	query.Set("min_block_number", strconv.FormatInt(fromBlock, 10))
	query.Set("max_block_number", strconv.FormatInt(toBlock, 10))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if setting.CryptoTronAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", setting.CryptoTronAPIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("TRON event HTTP status %d", resp.StatusCode)
	}
	var payload tronGridEventResponse
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	filtered := make([]tronGridEvent, 0, len(payload.Data))
	for _, event := range payload.Data {
		if event.BlockNumber >= fromBlock && event.BlockNumber <= toBlock {
			filtered = append(filtered, event)
		}
	}
	return filtered, nil
}
