package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

type SolanaScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewSolanaScanner(config setting.CryptoPaymentNetworkConfig) *SolanaScanner {
	return &SolanaScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *SolanaScanner) Network() string { return model.CryptoNetworkSolana }

func (s *SolanaScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoSolanaRPCURL) == "" {
		return fmt.Errorf("Solana RPC URL is not configured")
	}
	currentSlot, err := s.currentSlot(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromSlot := currentSlot - int64(s.config.Confirmations) - 500
	if err == nil && state.LastScannedBlock > 0 {
		fromSlot = state.LastScannedBlock + 1
	}
	if fromSlot < 0 {
		fromSlot = 0
	}
	maxSafe := currentSlot - int64(s.config.Confirmations) + 1
	if maxSafe < fromSlot {
		return nil
	}
	signatures, err := s.getSignatures(ctx)
	if err != nil {
		return err
	}
	for _, sig := range signatures {
		if sig.Err != nil || sig.Slot < fromSlot || sig.Slot > maxSafe {
			continue
		}
		tx, err := s.getTransaction(ctx, sig.Signature)
		if err != nil {
			return err
		}
		transfers, err := decodeSolanaTokenTransfers(sig.Signature, tx, s.config.ReceiveAddress, s.config.Contract, s.config.Decimals, currentSlot)
		if err != nil {
			return err
		}
		for _, transfer := range transfers {
			transfer.Network = s.Network()
			transfer.ObservedAt = time.Now()
			if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
				return err
			}
		}
	}
	return model.UpsertCryptoScannerState(s.Network(), maxSafe, maxSafe)
}

type solanaRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type solanaRPCResponse struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type solanaSignatureInfo struct {
	Signature string `json:"signature"`
	Slot      int64  `json:"slot"`
	Err       any    `json:"err"`
}

type solanaTransactionResult struct {
	Slot        int64                 `json:"slot"`
	BlockTime   int64                 `json:"blockTime"`
	Transaction solanaTransaction     `json:"transaction"`
	Meta        solanaTransactionMeta `json:"meta"`
}

type solanaTransaction struct {
	Message solanaMessage `json:"message"`
}

type solanaMessage struct {
	AccountKeys  []solanaAccountKey  `json:"accountKeys"`
	Instructions []solanaInstruction `json:"instructions"`
}

type solanaAccountKey struct {
	Pubkey string `json:"pubkey"`
}

type solanaTransactionMeta struct {
	PostTokenBalances []solanaTokenBalance     `json:"postTokenBalances"`
	InnerInstructions []solanaInnerInstruction `json:"innerInstructions"`
}

type solanaInnerInstruction struct {
	Index        int                 `json:"index"`
	Instructions []solanaInstruction `json:"instructions"`
}

type solanaTokenBalance struct {
	AccountIndex  int               `json:"accountIndex"`
	Mint          string            `json:"mint"`
	Owner         string            `json:"owner"`
	UITokenAmount solanaTokenAmount `json:"uiTokenAmount"`
}

type solanaTokenAmount struct {
	Amount string `json:"amount"`
}

type solanaInstruction struct {
	Program string                  `json:"program"`
	Parsed  solanaParsedInstruction `json:"parsed"`
}

type solanaParsedInstruction struct {
	Type string         `json:"type"`
	Info map[string]any `json:"info"`
}

type solanaTokenAccountInfo struct {
	Mint   string
	Owner  string
	Amount string
}

func decodeSolanaTokenTransfers(signature string, tx solanaTransactionResult, receiveAddress string, mint string, decimals int, currentSlot int64) ([]model.CryptoObservedTransfer, error) {
	receiveAddress = strings.TrimSpace(receiveAddress)
	mint = strings.TrimSpace(mint)
	tokenAccounts := make(map[string]solanaTokenAccountInfo, len(tx.Meta.PostTokenBalances))
	for _, balance := range tx.Meta.PostTokenBalances {
		if balance.AccountIndex < 0 || balance.AccountIndex >= len(tx.Transaction.Message.AccountKeys) {
			continue
		}
		tokenAccount := strings.TrimSpace(tx.Transaction.Message.AccountKeys[balance.AccountIndex].Pubkey)
		tokenAccounts[tokenAccount] = solanaTokenAccountInfo{
			Mint:   strings.TrimSpace(balance.Mint),
			Owner:  strings.TrimSpace(balance.Owner),
			Amount: strings.TrimSpace(balance.UITokenAmount.Amount),
		}
	}

	instructions := make([]solanaInstruction, 0, len(tx.Transaction.Message.Instructions))
	instructions = append(instructions, tx.Transaction.Message.Instructions...)
	for _, inner := range tx.Meta.InnerInstructions {
		instructions = append(instructions, inner.Instructions...)
	}

	transfers := make([]model.CryptoObservedTransfer, 0)
	for index, instruction := range instructions {
		if instruction.Program != "spl-token" && instruction.Program != "spl-token-2022" {
			continue
		}
		parsedType := instruction.Parsed.Type
		if parsedType != "transfer" && parsedType != "transferChecked" {
			continue
		}
		destination := stringFromAny(instruction.Parsed.Info["destination"])
		if destination == "" {
			continue
		}
		accountInfo := tokenAccounts[destination]
		transferMint := firstNonEmpty(stringFromAny(instruction.Parsed.Info["mint"]), accountInfo.Mint)
		if !strings.EqualFold(transferMint, mint) {
			continue
		}
		toAddress := firstNonEmpty(accountInfo.Owner, destination)
		if !strings.EqualFold(toAddress, receiveAddress) && !strings.EqualFold(destination, receiveAddress) {
			continue
		}
		amount := firstNonEmpty(
			stringFromAny(instruction.Parsed.Info["amount"]),
			stringFromMap(instruction.Parsed.Info, "tokenAmount", "amount"),
			accountInfo.Amount,
		)
		if amount == "" {
			continue
		}
		confirmations := int64(0)
		if currentSlot >= tx.Slot {
			confirmations = currentSlot - tx.Slot + 1
		}
		transfers = append(transfers, model.CryptoObservedTransfer{
			TxHash:          signature,
			LogIndex:        index,
			BlockNumber:     tx.Slot,
			BlockTimestamp:  tx.BlockTime,
			FromAddress:     firstNonEmpty(stringFromAny(instruction.Parsed.Info["authority"]), stringFromAny(instruction.Parsed.Info["source"])),
			ToAddress:       toAddress,
			TokenContract:   mint,
			TokenDecimals:   decimals,
			AmountBaseUnits: amount,
			Confirmations:   confirmations,
		})
	}
	return transfers, nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func stringFromMap(values map[string]any, key string, nestedKey string) string {
	nested, ok := values[key].(map[string]any)
	if !ok {
		return ""
	}
	return stringFromAny(nested[nestedKey])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *SolanaScanner) currentSlot(ctx context.Context) (int64, error) {
	var slot int64
	err := s.rpc(ctx, "getSlot", []interface{}{map[string]interface{}{"commitment": "confirmed"}}, &slot)
	return slot, err
}

func (s *SolanaScanner) getSignatures(ctx context.Context) ([]solanaSignatureInfo, error) {
	var signatures []solanaSignatureInfo
	err := s.rpc(ctx, "getSignaturesForAddress", []interface{}{
		s.config.ReceiveAddress,
		map[string]interface{}{"limit": 100, "commitment": "confirmed"},
	}, &signatures)
	return signatures, err
}

func (s *SolanaScanner) getTransaction(ctx context.Context, signature string) (solanaTransactionResult, error) {
	var tx solanaTransactionResult
	err := s.rpc(ctx, "getTransaction", []interface{}{
		signature,
		map[string]interface{}{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": 0},
	}, &tx)
	return tx, err
}

func (s *SolanaScanner) rpc(ctx context.Context, method string, params []interface{}, out interface{}) error {
	if params == nil {
		params = []interface{}{}
	}
	payload, err := common.Marshal(solanaRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.CryptoSolanaRPCURL, bytes.NewReader(payload))
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
		return fmt.Errorf("Solana RPC HTTP status %d", resp.StatusCode)
	}
	var envelope solanaRPCResponse
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("Solana RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	encoded, err := common.Marshal(envelope.Result)
	if err != nil {
		return err
	}
	return common.Unmarshal(encoded, out)
}
