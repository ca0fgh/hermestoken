package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const (
	// TRON reports log topics without the 0x prefix; normalizeTronHex strips it
	// either way so this constant compares against both shapes.
	tronTransferTopic = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	// A scanner that is days behind must not ask TronGrid for a months-wide
	// timestamp window in a single request; it walks forward in bounded steps
	// instead, resuming from the cursor on the next tick.
	tronMaxBlockSpan = 3000
	tronPageLimit    = 200
	// Deposits to one receive address inside a tronMaxBlockSpan window (~2.5h)
	// cannot plausibly fill this many pages. Hitting the cap means the window was
	// truncated, which must surface as an error rather than silently drop the
	// remainder — silent truncation past a cursor is an unrecoverable lost deposit.
	tronMaxPages = 25
)

type TronScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
	pool   *endpointPool
	// The configured base58 addresses, pre-decoded to the bare hex a node reports.
	// Comparing hex to hex keeps base58's case-sensitivity out of the hot path.
	contractHex string
	receiveHex  string
	configErr   error
}

func NewTronScanner(config setting.CryptoPaymentNetworkConfig) *TronScanner {
	scanner := &TronScanner{
		config: config,
		client: &http.Client{Timeout: 20 * time.Second},
		pool:   newEndpointPool(model.CryptoNetworkTronTRC20, setting.CryptoRPCEndpoints(model.CryptoNetworkTronTRC20)),
	}
	contractHex, err := tronHexFromAddress(config.Contract)
	if err != nil {
		scanner.configErr = fmt.Errorf("invalid TRON token contract %q: %w", config.Contract, err)
		return scanner
	}
	receiveHex, err := tronHexFromAddress(config.ReceiveAddress)
	if err != nil {
		scanner.configErr = fmt.Errorf("invalid TRON receive address %q: %w", config.ReceiveAddress, err)
		return scanner
	}
	scanner.contractHex = contractHex
	scanner.receiveHex = receiveHex
	return scanner
}

func (s *TronScanner) Network() string { return model.CryptoNetworkTronTRC20 }

func (s *TronScanner) ScanOnce(ctx context.Context) error {
	if s.configErr != nil {
		return s.configErr
	}
	if s.pool.size() == 0 {
		return fmt.Errorf("TRON RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	lastScanned := int64(0)
	if err == nil {
		lastScanned = state.LastScannedBlock
	}
	fromBlock := resumeFromBlock(s.Network(), lastScanned, currentBlock, currentBlock-int64(s.config.Confirmations)-40)
	if fromBlock < 1 {
		fromBlock = 1
	}
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if maxSafe < fromBlock {
		return nil
	}
	if maxSafe-fromBlock+1 > tronMaxBlockSpan {
		maxSafe = fromBlock + tronMaxBlockSpan - 1
	}

	fromTimestamp, err := s.blockTimestamp(ctx, fromBlock)
	if err != nil {
		return err
	}
	toTimestamp, err := s.blockTimestamp(ctx, maxSafe)
	if err != nil {
		return err
	}

	txIDs, err := s.discoverIncomingTransfers(ctx, fromTimestamp, toTimestamp)
	if err != nil {
		return err
	}

	lastScanned, scanErr := s.recordTransactions(ctx, txIDs, fromBlock, maxSafe, currentBlock)
	// Persist before surfacing scanErr. Progress made ahead of a failing request is
	// still progress, and discarding it is what pinned the Polygon scanner to one
	// doomed request for two months.
	if lastScanned >= fromBlock {
		if err := model.UpsertCryptoScannerState(s.Network(), lastScanned, maxSafe); err != nil {
			return err
		}
	}
	if reportScannerProgress(s.Network(), lastScanned, maxSafe, currentBlock) {
		s.pool.resetToPrimary()
	}
	return scanErr
}

// recordTransactions walks the candidate transactions in chronological order and
// returns the highest block it fully covered. Because discovery queried the whole
// window by address, a block with no candidate has no transfer in it — so every
// block below the transaction being worked on is genuinely done.
func (s *TronScanner) recordTransactions(ctx context.Context, txIDs []string, fromBlock int64, maxSafe int64, currentBlock int64) (int64, error) {
	lastScanned := fromBlock - 1
	for _, txID := range txIDs {
		info, err := s.transactionInfo(ctx, txID)
		if err != nil {
			return lastScanned, err
		}
		if info.BlockNumber < fromBlock || info.BlockNumber > maxSafe {
			continue
		}
		for _, transfer := range s.decodeIncomingTransfers(txID, info, currentBlock) {
			if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
				return lastScanned, err
			}
		}
		// Only blocks strictly below this one are provably complete: another
		// candidate may share this block. Re-scanning a block is free — the
		// transfer table is keyed on (network, tx_hash, log_index).
		if info.BlockNumber-1 > lastScanned {
			lastScanned = info.BlockNumber - 1
		}
	}
	return maxSafe, nil
}

func (s *TronScanner) decodeIncomingTransfers(txID string, info tronTransactionInfo, currentBlock int64) []model.CryptoObservedTransfer {
	transfers := make([]model.CryptoObservedTransfer, 0, 1)
	for index, entry := range info.Log {
		if normalizeTronHex(entry.Address) != s.contractHex {
			continue
		}
		if len(entry.Topics) < 3 || normalizeTronHex(entry.Topics[0]) != tronTransferTopic {
			continue
		}
		if tronTopicToHex(entry.Topics[2]) != s.receiveHex {
			continue
		}
		amount := new(big.Int)
		if _, ok := amount.SetString(normalizeTronHex(entry.Data), 16); !ok {
			// A real Transfer always carries a parseable amount. Skipping a log we
			// cannot read keeps one malformed event from wedging the cursor forever,
			// but it must never be silent — a dropped deposit has to be findable.
			common.SysLog(fmt.Sprintf("crypto scanner skipped unreadable TRON transfer log: tx=%s index=%d data=%q", txID, index, entry.Data))
			continue
		}
		fromAddress, err := tronAddressFromHex(tronTopicToHex(entry.Topics[1]))
		if err != nil {
			fromAddress = ""
		}
		transfers = append(transfers, model.CryptoObservedTransfer{
			Network:         s.Network(),
			TxHash:          txID,
			LogIndex:        index,
			BlockNumber:     info.BlockNumber,
			BlockTimestamp:  info.BlockTimeStamp / 1000,
			FromAddress:     fromAddress,
			ToAddress:       s.config.ReceiveAddress,
			TokenContract:   s.config.Contract,
			TokenDecimals:   s.config.Decimals,
			AmountBaseUnits: amount.String(),
			Confirmations:   currentBlock - info.BlockNumber + 1,
			ObservedAt:      time.Now(),
		})
	}
	return transfers
}

// discoverIncomingTransfers asks TronGrid for the transfers of one token to one
// address. The previous implementation asked for every Transfer event emitted by
// the USDT contract and filtered client-side — on the busiest token contract in
// existence that page is truncated at its limit, and the cursor advanced past the
// dropped remainder anyway.
func (s *TronScanner) discoverIncomingTransfers(ctx context.Context, fromTimestamp int64, toTimestamp int64) ([]string, error) {
	path := "/v1/accounts/" + url.PathEscape(s.config.ReceiveAddress) + "/transactions/trc20"
	query := url.Values{}
	query.Set("contract_address", s.config.Contract)
	query.Set("only_confirmed", "true")
	query.Set("limit", strconv.Itoa(tronPageLimit))
	query.Set("order_by", "block_timestamp,asc")
	query.Set("min_timestamp", strconv.FormatInt(fromTimestamp, 10))
	query.Set("max_timestamp", strconv.FormatInt(toTimestamp, 10))

	seen := make(map[string]bool)
	txIDs := make([]string, 0, 8)
	fingerprint := ""
	for page := 0; page < tronMaxPages; page++ {
		if fingerprint != "" {
			query.Set("fingerprint", fingerprint)
		}

		var payload tronTRC20Response
		if err := s.getJSON(ctx, path, query, &payload); err != nil {
			return nil, err
		}
		for _, row := range payload.Data {
			// base58 is case-sensitive; an EqualFold here would accept a different
			// address entirely.
			if strings.TrimSpace(row.To) != strings.TrimSpace(s.config.ReceiveAddress) {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(row.TokenInfo.Address), strings.TrimSpace(s.config.Contract)) {
				continue
			}
			txID := strings.TrimSpace(row.TransactionID)
			if txID == "" || seen[txID] {
				continue
			}
			seen[txID] = true
			txIDs = append(txIDs, txID)
		}
		fingerprint = strings.TrimSpace(payload.Meta.Fingerprint)
		if fingerprint == "" || len(payload.Data) == 0 {
			return txIDs, nil
		}
	}
	return nil, fmt.Errorf("TRON transfer discovery exceeded %d pages for blocks in [%d, %d]", tronMaxPages, fromTimestamp, toTimestamp)
}

type tronTRC20Response struct {
	Data []tronTRC20Transfer `json:"data"`
	Meta struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"meta"`
}

type tronTRC20Transfer struct {
	TransactionID  string `json:"transaction_id"`
	BlockTimestamp int64  `json:"block_timestamp"`
	From           string `json:"from"`
	To             string `json:"to"`
	Type           string `json:"type"`
	Value          string `json:"value"`
	TokenInfo      struct {
		Address  string `json:"address"`
		Decimals int    `json:"decimals"`
		Symbol   string `json:"symbol"`
	} `json:"token_info"`
}

type tronTransactionInfo struct {
	ID             string            `json:"id"`
	BlockNumber    int64             `json:"blockNumber"`
	BlockTimeStamp int64             `json:"blockTimeStamp"`
	Log            []tronContractLog `json:"log"`
}

type tronContractLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// Verify asks TRON whether the configured token contract is actually deployed.
//
// On TRON that single question also settles which chain the endpoint is on: mainnet
// USDT does not exist on Nile or Shasta, so a testnet endpoint fails this check for
// the same reason a nonexistent contract does. Production shipped exactly that —
// a contract address deployed on no TRON chain at all — and the scanner's only
// symptom was finding nothing, forever.
func (s *TronScanner) Verify(ctx context.Context) setting.CryptoNetworkHealth {
	network := s.Network()
	if s.configErr != nil {
		return healthMismatch(network, s.configErr.Error())
	}
	if s.pool.size() == 0 {
		return healthUnknown(network, "no RPC endpoint is configured")
	}

	var contract struct {
		ContractAddress string `json:"contract_address"`
		Bytecode        string `json:"bytecode"`
	}
	err := s.postJSON(ctx, "/wallet/getcontract", map[string]interface{}{
		"value":   s.config.Contract,
		"visible": true,
	}, &contract)
	if err != nil {
		return healthUnknown(network, "could not read the token contract: "+err.Error())
	}
	// A TRON node answers 200 with an empty object for an address that holds no
	// contract, so "no error" is not the same as "it is there".
	if strings.TrimSpace(contract.Bytecode) == "" && strings.TrimSpace(contract.ContractAddress) == "" {
		return healthMismatch(network, fmt.Sprintf(
			"no contract is deployed at %s on this TRON chain, so it can never emit the transfer a deposit is matched by", s.config.Contract))
	}

	decimals, err := s.tokenDecimals(ctx)
	if err != nil {
		// Only a contradiction disqualifies a network. An unanswered question is not one.
		common.SysLog(fmt.Sprintf("crypto payment network %s: could not read token decimals (%s), continuing", network, err.Error()))
		return healthOK(network, fmt.Sprintf("contract %s is deployed (its decimals could not be read)", s.config.Contract))
	}
	if decimals != s.config.Decimals {
		return healthMismatch(network, decimalsMismatchDetail(s.config.Contract, decimals, s.config.Decimals))
	}
	return healthOK(network, fmt.Sprintf("contract %s is deployed and carries the configured %d decimals", s.config.Contract, decimals))
}

// tokenDecimals calls decimals() on the TRC-20 contract. triggerconstantcontract
// runs it read-only, so this needs no key and costs no energy.
func (s *TronScanner) tokenDecimals(ctx context.Context) (int, error) {
	var result struct {
		ConstantResult []string `json:"constant_result"`
	}
	err := s.postJSON(ctx, "/wallet/triggerconstantcontract", map[string]interface{}{
		"owner_address":     s.config.ReceiveAddress,
		"contract_address":  s.config.Contract,
		"function_selector": "decimals()",
		"visible":           true,
	}, &result)
	if err != nil {
		return 0, err
	}
	if len(result.ConstantResult) == 0 || strings.TrimSpace(result.ConstantResult[0]) == "" {
		return 0, fmt.Errorf("TRON contract %s returned no decimals", s.config.Contract)
	}
	decimals, ok := new(big.Int).SetString(strings.TrimSpace(result.ConstantResult[0]), 16)
	if !ok || !decimals.IsInt64() {
		return 0, fmt.Errorf("TRON contract %s returned an unreadable decimals value %q", s.config.Contract, result.ConstantResult[0])
	}
	return int(decimals.Int64()), nil
}

func (s *TronScanner) currentBlock(ctx context.Context) (int64, error) {
	var payload struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := s.postJSON(ctx, "/wallet/getnowblock", map[string]interface{}{}, &payload); err != nil {
		return 0, err
	}
	if payload.BlockHeader.RawData.Number <= 0 {
		return 0, fmt.Errorf("TRON current block response missing block number")
	}
	return payload.BlockHeader.RawData.Number, nil
}

func (s *TronScanner) blockTimestamp(ctx context.Context, blockNumber int64) (int64, error) {
	var payload struct {
		BlockHeader struct {
			RawData struct {
				Timestamp int64 `json:"timestamp"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := s.postJSON(ctx, "/wallet/getblockbynum", map[string]interface{}{"num": blockNumber}, &payload); err != nil {
		return 0, err
	}
	if payload.BlockHeader.RawData.Timestamp <= 0 {
		return 0, fmt.Errorf("TRON block %d response missing timestamp", blockNumber)
	}
	return payload.BlockHeader.RawData.Timestamp, nil
}

func (s *TronScanner) transactionInfo(ctx context.Context, txID string) (tronTransactionInfo, error) {
	var info tronTransactionInfo
	if err := s.postJSON(ctx, "/wallet/gettransactioninfobyid", map[string]interface{}{"value": txID}, &info); err != nil {
		return tronTransactionInfo{}, err
	}
	if info.BlockNumber <= 0 {
		return tronTransactionInfo{}, fmt.Errorf("TRON transaction %s has no block number", txID)
	}
	return info, nil
}

func (s *TronScanner) postJSON(ctx context.Context, path string, body map[string]interface{}, out interface{}) error {
	payload, err := common.Marshal(body)
	if err != nil {
		return err
	}
	return s.pool.do(func(endpoint string) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		return s.do(req, out)
	})
}

func (s *TronScanner) getJSON(ctx context.Context, path string, query url.Values, out interface{}) error {
	return s.pool.do(func(endpoint string) error {
		target := endpoint + path
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		return s.do(req, out)
	})
}

func (s *TronScanner) do(req *http.Request, out interface{}) error {
	if setting.CryptoTronAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", setting.CryptoTronAPIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return endpointUnavailable(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return endpointUnavailable(fmt.Errorf("TRON API HTTP status %d for %s", resp.StatusCode, req.URL.Path))
	}
	return common.DecodeJson(resp.Body, out)
}
