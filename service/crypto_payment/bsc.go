package crypto_payment

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const (
	bscTransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	bscChainID       = 56
)

type BSCScanner struct {
	config setting.CryptoPaymentNetworkConfig
	rpc    *evmRPCClient
	// See PolygonScanner.blockSpan. BSC's RPC currently serves the full opening
	// span, so this never shrinks in practice — but it is the same code shape
	// that silently killed Polygon, and a provider's cap is not ours to assume.
	blockSpan int64
}

func NewBSCScanner(config setting.CryptoPaymentNetworkConfig) *BSCScanner {
	return &BSCScanner{
		config:    config,
		rpc:       newEVMRPCClient(model.CryptoNetworkBSCERC20, setting.CryptoRPCEndpoints(model.CryptoNetworkBSCERC20)),
		blockSpan: evmMaxBlockSpan,
	}
}

func (s *BSCScanner) Network() string { return model.CryptoNetworkBSCERC20 }

func (s *BSCScanner) Verify(ctx context.Context) setting.CryptoNetworkHealth {
	return verifyEVMNetwork(ctx, s.Network(), s.rpc, bscChainID, s.config)
}

func (s *BSCScanner) ScanOnce(ctx context.Context) error {
	if s.rpc.pool.size() == 0 {
		return fmt.Errorf("BSC RPC URL is not configured")
	}
	currentBlock, err := s.rpc.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	lastScanned := int64(0)
	if err == nil {
		lastScanned = state.LastScannedBlock
	}
	fromBlock := resumeFromBlock(s.Network(), lastScanned, currentBlock, currentBlock-int64(s.config.Confirmations)-30)
	if fromBlock < 0 {
		fromBlock = 0
	}
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if maxSafe < fromBlock {
		return nil
	}
	lastScanned, scanErr := scanEVMRange(ctx, &s.blockSpan, fromBlock, maxSafe,
		func(ctx context.Context, from int64, to int64) ([]bscRPCLog, error) {
			return s.rpc.transferLogs(ctx, s.config.Contract, s.config.ReceiveAddress, from, to)
		},
		func(logs []bscRPCLog) error {
			return recordEVMTransferLogs(ctx, s.Network(), s.config, s.rpc, logs, currentBlock)
		},
	)
	if lastScanned >= fromBlock {
		if err := model.UpsertCryptoScannerState(s.Network(), lastScanned, maxSafe); err != nil {
			return err
		}
	}
	if reportScannerProgress(s.Network(), lastScanned, maxSafe, currentBlock) {
		s.rpc.pool.resetToPrimary()
		s.blockSpan = evmMaxBlockSpan
	}
	return scanErr
}

func (s *BSCScanner) blockTimestamp(ctx context.Context, blockNumber int64) (int64, error) {
	return s.rpc.blockTimestamp(ctx, blockNumber)
}

// recordEVMTransferLogs turns the Transfer logs of one chunk into observed transfers.
// BSC and Polygon differ only in which chain they ask; what an incoming USDT
// transfer means is the same on both.
func recordEVMTransferLogs(
	ctx context.Context,
	network string,
	config setting.CryptoPaymentNetworkConfig,
	rpc *evmRPCClient,
	logs []bscRPCLog,
	currentBlock int64,
) error {
	blockTimestamps := make(map[int64]int64)
	for _, item := range logs {
		transfer, err := decodeBSCTransferLog(item, config.Decimals)
		if err != nil {
			return err
		}
		if !strings.EqualFold(transfer.ToAddress, config.ReceiveAddress) {
			continue
		}
		blockTimestamp, ok := blockTimestamps[transfer.BlockNumber]
		if !ok {
			blockTimestamp, err = rpc.blockTimestamp(ctx, transfer.BlockNumber)
			if err != nil {
				return err
			}
			blockTimestamps[transfer.BlockNumber] = blockTimestamp
		}
		transfer.Network = network
		transfer.TokenContract = config.Contract
		transfer.BlockTimestamp = blockTimestamp
		transfer.Confirmations = currentBlock - transfer.BlockNumber + 1
		transfer.ObservedAt = time.Now()
		if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
			return err
		}
	}
	return nil
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
