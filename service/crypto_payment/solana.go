package crypto_payment

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const (
	solanaSignaturePageLimit = 1000
	// A receive address accumulates signatures slowly, so reaching the cursor
	// always takes a page or two. Blowing through this many means the history is
	// deeper than we can prove we covered, which must surface rather than let the
	// cursor jump a gap.
	solanaMaxSignaturePages = 20
)

type SolanaScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewSolanaScanner(config setting.CryptoPaymentNetworkConfig) *SolanaScanner {
	return &SolanaScanner{config: config, client: &http.Client{Timeout: 20 * time.Second}}
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

	addresses, err := s.signatureAddresses(ctx)
	if err != nil {
		return err
	}
	signatures, err := s.collectSignatures(ctx, addresses, fromSlot, maxSafe)
	if err != nil {
		return err
	}

	lastScanned, scanErr := s.recordSignatures(ctx, signatures, fromSlot, maxSafe, currentSlot)
	// Persist before surfacing scanErr: the signatures are walked oldest-first, so
	// everything below the failing slot is genuinely done, and throwing that away
	// is what turns one bad response into a permanent stall.
	if lastScanned >= fromSlot {
		if err := model.UpsertCryptoScannerState(s.Network(), lastScanned, maxSafe); err != nil {
			return err
		}
	}
	return scanErr
}

// signatureAddresses returns every address whose signature history can contain an
// incoming deposit.
//
// An SPL transfer credits the receiver's associated token account, and the owner
// wallet is not among the transaction's account keys — so asking for the wallet's
// signatures, as this scanner used to, returns nothing at all for a deposit. The
// token accounts are the real subject. The wallet is still included because the
// transfer that first *creates* the token account does list the owner, and that
// one deposit would otherwise be missed.
func (s *SolanaScanner) signatureAddresses(ctx context.Context) ([]string, error) {
	var result struct {
		Value []struct {
			Pubkey string `json:"pubkey"`
		} `json:"value"`
	}
	err := s.rpc(ctx, "getTokenAccountsByOwner", []interface{}{
		s.config.ReceiveAddress,
		map[string]interface{}{"mint": s.config.Contract},
		map[string]interface{}{"encoding": "jsonParsed", "commitment": "confirmed"},
	}, &result)
	if err != nil {
		return nil, err
	}
	addresses := make([]string, 0, len(result.Value)+1)
	addresses = append(addresses, s.config.ReceiveAddress)
	for _, account := range result.Value {
		if pubkey := strings.TrimSpace(account.Pubkey); pubkey != "" {
			addresses = append(addresses, pubkey)
		}
	}
	return addresses, nil
}

func (s *SolanaScanner) collectSignatures(ctx context.Context, addresses []string, fromSlot int64, maxSafe int64) ([]solanaSignatureInfo, error) {
	seen := make(map[string]bool)
	collected := make([]solanaSignatureInfo, 0, 8)
	for _, address := range addresses {
		found, err := s.signaturesForAddress(ctx, address, fromSlot, maxSafe)
		if err != nil {
			return nil, err
		}
		for _, signature := range found {
			if seen[signature.Signature] {
				continue
			}
			seen[signature.Signature] = true
			collected = append(collected, signature)
		}
	}
	// Oldest first, so a failure part-way through leaves a contiguous scanned
	// prefix that the cursor can safely record.
	sort.Slice(collected, func(i, j int) bool { return collected[i].Slot < collected[j].Slot })
	return collected, nil
}

// signaturesForAddress pages backwards from the tip until it reaches the cursor.
// getSignaturesForAddress returns newest-first and cannot be given a slot range,
// so the only way to prove full coverage of [fromSlot, maxSafe] is to walk back
// past fromSlot. The old code took a single unpaged page of 100 and then advanced
// the cursor to the chain tip regardless — anything beyond that page was skipped
// permanently, with no path to ever revisit it.
func (s *SolanaScanner) signaturesForAddress(ctx context.Context, address string, fromSlot int64, maxSafe int64) ([]solanaSignatureInfo, error) {
	collected := make([]solanaSignatureInfo, 0, 8)
	before := ""
	for page := 0; page < solanaMaxSignaturePages; page++ {
		options := map[string]interface{}{"limit": solanaSignaturePageLimit, "commitment": "confirmed"}
		if before != "" {
			options["before"] = before
		}
		var signatures []solanaSignatureInfo
		if err := s.rpc(ctx, "getSignaturesForAddress", []interface{}{address, options}, &signatures); err != nil {
			return nil, err
		}
		if len(signatures) == 0 {
			return collected, nil
		}
		reachedCursor := false
		for _, signature := range signatures {
			if signature.Slot < fromSlot {
				reachedCursor = true
				continue
			}
			if signature.Err != nil || signature.Slot > maxSafe {
				continue
			}
			collected = append(collected, signature)
		}
		if reachedCursor || len(signatures) < solanaSignaturePageLimit {
			return collected, nil
		}
		before = signatures[len(signatures)-1].Signature
	}
	return nil, fmt.Errorf("Solana signature history for %s is deeper than %d pages above slot %d", address, solanaMaxSignaturePages, fromSlot)
}

func (s *SolanaScanner) recordSignatures(ctx context.Context, signatures []solanaSignatureInfo, fromSlot int64, maxSafe int64, currentSlot int64) (int64, error) {
	lastScanned := fromSlot - 1
	for _, signature := range signatures {
		tx, found, err := s.getTransaction(ctx, signature.Signature)
		if err != nil {
			return lastScanned, err
		}
		if !found {
			// A pruned or unavailable transaction must not wedge the cursor on a
			// request that can never succeed, but it also must not vanish quietly.
			common.SysLog("crypto scanner could not load Solana transaction: " + signature.Signature)
			continue
		}
		transfers, err := decodeSolanaTokenTransfers(signature.Signature, tx, s.config.ReceiveAddress, s.config.Contract, s.config.Decimals, currentSlot)
		if err != nil {
			return lastScanned, err
		}
		for _, transfer := range transfers {
			transfer.Network = s.Network()
			transfer.ObservedAt = time.Now()
			if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
				return lastScanned, err
			}
		}
		// Only slots strictly below this one are provably complete: another
		// signature may share this slot.
		if signature.Slot-1 > lastScanned {
			lastScanned = signature.Slot - 1
		}
	}
	return maxSafe, nil
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

// getTransaction reports found=false for a slot the node will not serve (pruned,
// or beyond its history), which is a skip rather than a failure.
func (s *SolanaScanner) getTransaction(ctx context.Context, signature string) (solanaTransactionResult, bool, error) {
	var tx solanaTransactionResult
	err := s.rpc(ctx, "getTransaction", []interface{}{
		signature,
		map[string]interface{}{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": 0},
	}, &tx)
	if err != nil {
		return solanaTransactionResult{}, false, err
	}
	return tx, tx.Slot > 0, nil
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
